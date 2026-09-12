// File purpose: startup wiring for P1.2 rule chain replay persistence.
// Core logic: install the replay recorder only when retention is explicitly enabled.
// Key notes: this must be opt-in. Replay retains raw node input, which is a second copy of
// payload data; installing the recorder unconditionally would silently start persisting
// payloads for every deployment. When disabled we explicitly set the recorder to nil so a
// previously installed instance cannot linger.

package app

import (
	"aetherlink-iot/backend/internal/service"

	"github.com/sirupsen/logrus"
)

// WithRuleChainReplayPersistence 按配置装配规则链回放留存。
func WithRuleChainReplayPersistence() Option {
	return func(app *Application) error {
		service.InstallRuleChainReplayPersistence()
		if service.RuleChainReplayRetentionEnabled() {
			logrus.Warn("rule chain replay retention is enabled: raw node input will be persisted")
		}
		return nil
	}
}
