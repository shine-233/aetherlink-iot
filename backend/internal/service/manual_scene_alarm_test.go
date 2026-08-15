package service

import (
	"testing"

	"aetherlink-iot/backend/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stubActiveSceneExecutionLog(t *testing.T, capture func(*model.SceneLog) error) {
	t.Helper()

	oldPersist := persistActiveSceneExecutionLogFn
	persistActiveSceneExecutionLogFn = capture
	t.Cleanup(func() { persistActiveSceneExecutionLogFn = oldPersist })
}

func stubManualSceneActions(t *testing.T, actions []model.ActionInfo) {
	t.Helper()

	oldLookup := getActionInfoListBySceneID
	getActionInfoListBySceneID = func(sceneIDs []string) ([]model.ActionInfo, error) {
		require.Equal(t, []string{"scene-1"}, sceneIDs)
		return actions, nil
	}
	t.Cleanup(func() { getActionInfoListBySceneID = oldLookup })
}

func TestActiveSceneExecuteReturnsActionFailureAfterPersistingFailureLog(t *testing.T) {
	stubManualSceneActions(t, []model.ActionInfo{{
		SceneAutomationID: "scene-1",
		ActionType:        "unsupported",
	}})
	var captured *model.SceneLog
	stubActiveSceneExecutionLog(t, func(log *model.SceneLog) error {
		captured = log
		return nil
	})

	err := (&Automate{}).ActiveSceneExecute("scene-1", "tenant-1")
	require.ErrorContains(t, err, "unsupported automate action")
	require.NotNil(t, captured)
	assert.Equal(t, "scene-1", captured.SceneID)
	assert.Equal(t, "tenant-1", captured.TenantID)
	assert.Equal(t, executionResultFailure, captured.ExecutionResult)
	assert.Contains(t, captured.Detail, "unsupported automate action")
}

func TestActiveSceneExecuteRunsAlarmWithoutAutomationCache(t *testing.T) {
	alarmConfigID := "alarm-config-1"
	stubManualSceneActions(t, []model.ActionInfo{{
		SceneAutomationID: "scene-1",
		ActionType:        model.AUTOMATE_ACTION_TYPE_ALARM,
		ActionTarget:      &alarmConfigID,
	}})

	oldCachedAlarm := executeCachedAutomationAlarm
	cachedAlarmCalled := false
	executeCachedAutomationAlarm = func(string, string) (bool, string, string) {
		cachedAlarmCalled = true
		return false, "", "alarm cache does not exist"
	}
	t.Cleanup(func() { executeCachedAutomationAlarm = oldCachedAlarm })

	oldDirectAlarm := executeDirectManualSceneAlarm
	directAlarmCalled := false
	executeDirectManualSceneAlarm = func(gotConfigID, content, sceneID, groupID string, deviceIDs []string) (bool, string, string) {
		directAlarmCalled = true
		assert.Equal(t, alarmConfigID, gotConfigID)
		assert.Equal(t, manualSceneAlarmContent, content)
		assert.Equal(t, "scene-1", sceneID)
		assert.Equal(t, "scene-1", groupID)
		assert.Empty(t, deviceIDs)
		return true, "manual alarm", ""
	}
	t.Cleanup(func() { executeDirectManualSceneAlarm = oldDirectAlarm })

	var captured *model.SceneLog
	stubActiveSceneExecutionLog(t, func(log *model.SceneLog) error {
		captured = log
		return nil
	})

	require.NoError(t, (&Automate{}).ActiveSceneExecute("scene-1", "tenant-1"))
	assert.True(t, directAlarmCalled)
	assert.False(t, cachedAlarmCalled)
	require.NotNil(t, captured)
	assert.Equal(t, executionResultSuccess, captured.ExecutionResult)
	assert.Contains(t, captured.Detail, "manual alarm")
}
