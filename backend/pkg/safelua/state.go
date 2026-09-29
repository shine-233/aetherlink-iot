package safelua

import (
	lua "github.com/yuin/gopher-lua"
)

// pooledState is a sandboxed LState plus a snapshot of everything a script
// can reach and mutate: every table reachable from the globals (library
// tables, the json module, their metatables), the Env of reachable
// functions, the per-type builtin metatables and the thread environment.
type pooledState struct {
	L        *lua.LState
	env      *lua.LTable
	tables   []tableSnapshot
	funcEnvs []funcEnvSnapshot
	typeMts  []typeMtSnapshot
}

type tableSnapshot struct {
	tb   *lua.LTable
	kv   map[lua.LValue]lua.LValue
	meta lua.LValue
}

type funcEnvSnapshot struct {
	fn  *lua.LFunction
	env *lua.LTable
}

type typeMtSnapshot struct {
	probe lua.LValue
	mt    lua.LValue
}

// builtinTypeProbes are values whose type-wide metatable lives in the global
// state (strings share one metatable, for instance). debug.setmetatable can
// change these, so they are snapshotted as well.
func builtinTypeProbes(L *lua.LState) []lua.LValue {
	thread, _ := L.NewThread()
	return []lua.LValue{
		lua.LNil, lua.LTrue, lua.LNumber(0), lua.LString(""),
		L.NewFunction(func(*lua.LState) int { return 0 }),
		thread,
		lua.LChannel(nil),
	}
}

func newPooledState() (*pooledState, error) {
	L := lua.NewState()
	jsonModule, err := setupSandboxWithModule(L)
	if err != nil {
		L.Close()
		return nil, err
	}
	st := &pooledState{L: L, env: L.Env}
	st.snapshot(jsonModule)
	return st, nil
}

func setupSandboxWithModule(L *lua.LState) (lua.LValue, error) {
	if err := setupSandbox(L); err != nil {
		return nil, err
	}
	// The allowed require returns the json module table; fetch it through the
	// public surface so it is included in the snapshot.
	if err := L.CallByParam(lua.P{Fn: L.GetGlobal("require"), NRet: 1, Protect: true}, lua.LString("json")); err != nil {
		return nil, err
	}
	module := L.Get(-1)
	L.Pop(1)
	return module, nil
}

func (st *pooledState) snapshot(extraRoots ...lua.LValue) {
	visitedTables := map[*lua.LTable]bool{}
	visitedFuncs := map[*lua.LFunction]bool{}
	var walk func(v lua.LValue)
	walk = func(v lua.LValue) {
		switch value := v.(type) {
		case *lua.LTable:
			if visitedTables[value] {
				return
			}
			visitedTables[value] = true
			snap := tableSnapshot{tb: value, kv: map[lua.LValue]lua.LValue{}, meta: value.Metatable}
			var children []lua.LValue
			value.ForEach(func(k, val lua.LValue) {
				snap.kv[k] = val
				children = append(children, k, val)
			})
			st.tables = append(st.tables, snap)
			for _, child := range children {
				walk(child)
			}
			if value.Metatable != nil {
				walk(value.Metatable)
			}
		case *lua.LFunction:
			if visitedFuncs[value] {
				return
			}
			visitedFuncs[value] = true
			st.funcEnvs = append(st.funcEnvs, funcEnvSnapshot{fn: value, env: value.Env})
			if value.Env != nil {
				walk(value.Env)
			}
		}
	}
	walk(st.L.G.Global)
	walk(st.env)
	for _, root := range extraRoots {
		walk(root)
	}
	for _, probe := range builtinTypeProbes(st.L) {
		mt := st.L.GetMetatable(probe)
		st.typeMts = append(st.typeMts, typeMtSnapshot{probe: probe, mt: mt})
		walk(mt)
	}
}

// reset restores the pristine snapshot. It returns false when the state cannot
// be proven clean, in which case the caller must discard it.
func (st *pooledState) reset() (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	L := st.L
	L.SetTop(0)
	L.Env = st.env
	for _, snap := range st.tables {
		restoreTable(snap)
	}
	for _, fe := range st.funcEnvs {
		fe.fn.Env = fe.env
	}
	for _, tm := range st.typeMts {
		if L.GetMetatable(tm.probe) != tm.mt {
			L.SetMetatable(tm.probe, tm.mt)
		}
	}
	return true
}

func restoreTable(snap tableSnapshot) {
	tb := snap.tb
	var extra []lua.LValue
	tb.ForEach(func(k, v lua.LValue) {
		if _, known := snap.kv[k]; !known {
			extra = append(extra, k)
		}
	})
	for _, k := range extra {
		tb.RawSet(k, lua.LNil)
	}
	for k, v := range snap.kv {
		if tb.RawGet(k) != v {
			tb.RawSet(k, v)
		}
	}
	tb.Metatable = snap.meta
}
