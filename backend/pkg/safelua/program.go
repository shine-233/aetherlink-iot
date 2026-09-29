package safelua

import (
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"runtime"
	"strings"
	"sync"

	lua "github.com/yuin/gopher-lua"
	"github.com/yuin/gopher-lua/parse"
)

// chunkName matches what LState.DoString used, so syntax/runtime error
// messages keep their previous "<string>:line:" prefix.
const chunkName = "<string>"

const (
	defaultMaxPrograms = 512
	maxIdleStatesCap   = 16
)

// Program is an immutable compiled script plus a bounded pool of sandboxed
// states dedicated to it.
type Program struct {
	proto      *lua.FunctionProto
	compileErr error
	idle       chan *pooledState

	mu     sync.Mutex
	closed bool
}

// Compile parses and compiles code once. A syntax error is returned as the
// same *lua.ApiError (ApiErrorSyntax) that DoString used to produce.
func Compile(code string) (*Program, error) {
	p := newProgram(code)
	if p.compileErr != nil {
		return nil, p.compileErr
	}
	return p, nil
}

func newProgram(code string) *Program {
	idle := runtime.GOMAXPROCS(0)
	if idle > maxIdleStatesCap {
		idle = maxIdleStatesCap
	}
	p := &Program{idle: make(chan *pooledState, idle)}
	chunk, err := parse.Parse(strings.NewReader(code), chunkName)
	if err == nil {
		p.proto, err = lua.Compile(chunk, chunkName)
	}
	if err != nil {
		p.compileErr = &lua.ApiError{Type: lua.ApiErrorSyntax, Object: lua.LString(err.Error()), Cause: err}
	}
	return p
}

func (p *Program) acquire() (*pooledState, error) {
	if p.compileErr != nil {
		return nil, p.compileErr
	}
	select {
	case st := <-p.idle:
		return st, nil
	default:
		return newPooledState()
	}
}

// release returns a state to the pool after restoring its pristine snapshot.
// Unhealthy states (errors, timeouts, panics) and states of closed programs
// are discarded, so an interrupted call stack is never reused.
func (p *Program) release(st *pooledState, healthy bool) {
	if st == nil {
		return
	}
	st.L.RemoveContext()
	if healthy {
		healthy = st.reset()
	}
	if !healthy {
		st.L.Close()
		return
	}
	// The closed check and the enqueue share one critical section with Close's
	// drain, so a state released concurrently with Close is never stranded
	// (unclosed) in the idle channel of a dropped program.
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		st.L.Close()
		return
	}
	select {
	case p.idle <- st:
	default:
		st.L.Close()
	}
}

// Close releases pooled states. In-flight runs close their state on release.
func (p *Program) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	for {
		select {
		case st := <-p.idle:
			st.L.Close()
		default:
			return
		}
	}
}

// IdleStates reports the number of pooled states (tests and diagnostics).
func (p *Program) IdleStates() int { return len(p.idle) }

// Cache is a bounded LRU of compiled programs keyed by (key, content hash).
type Cache struct {
	mu    sync.Mutex
	max   int
	ll    *list.List
	items map[string]*list.Element
	byKey map[string]string // caller key -> current cache id
}

type cacheEntry struct {
	id      string
	key     string
	code    string
	program *Program
}

// NewCache creates a program cache holding at most maxPrograms entries.
func NewCache(maxPrograms int) *Cache {
	if maxPrograms <= 0 {
		maxPrograms = defaultMaxPrograms
	}
	return &Cache{
		max:   maxPrograms,
		ll:    list.New(),
		items: make(map[string]*list.Element),
		byKey: make(map[string]string),
	}
}

var defaultCache = NewCache(defaultMaxPrograms)

func contentHash(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

// Program returns the compiled program for (key, code), compiling on miss.
func (c *Cache) Program(key, code string) *Program {
	if key != "" {
		// Hot path: the key's current program has identical source, so skip
		// hashing the whole script per message (string equality is a memcmp).
		c.mu.Lock()
		if id, ok := c.byKey[key]; ok {
			if el, ok := c.items[id]; ok {
				entry := el.Value.(*cacheEntry)
				if entry.code == code {
					c.ll.MoveToFront(el)
					c.mu.Unlock()
					return entry.program
				}
			}
		}
		c.mu.Unlock()
	}
	id := key + "\x00" + contentHash(code)
	c.mu.Lock()
	if el, ok := c.items[id]; ok {
		c.ll.MoveToFront(el)
		p := el.Value.(*cacheEntry).program
		c.mu.Unlock()
		return p
	}
	c.mu.Unlock()

	// Compile outside the lock; a concurrent duplicate compile is harmless.
	compiled := newProgram(code)

	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[id]; ok {
		c.ll.MoveToFront(el)
		return el.Value.(*cacheEntry).program
	}
	if key != "" {
		if oldID, ok := c.byKey[key]; ok && oldID != id {
			c.removeLocked(oldID)
		}
		c.byKey[key] = id
	}
	c.items[id] = c.ll.PushFront(&cacheEntry{id: id, key: key, code: code, program: compiled})
	for c.ll.Len() > c.max {
		c.removeLocked(c.ll.Back().Value.(*cacheEntry).id)
	}
	return compiled
}

// Execute compiles (or reuses) the program and runs it once.
func (c *Cache) Execute(ctx context.Context, key, code string, msg []byte, topic string) (string, error) {
	return c.Program(key, code).Run(ctx, msg, topic)
}

// Invalidate drops the program currently stored under key.
func (c *Cache) Invalidate(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if id, ok := c.byKey[key]; ok {
		c.removeLocked(id)
	}
}

// Len reports the number of cached programs.
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ll.Len()
}

func (c *Cache) removeLocked(id string) {
	el, ok := c.items[id]
	if !ok {
		return
	}
	entry := el.Value.(*cacheEntry)
	c.ll.Remove(el)
	delete(c.items, id)
	if entry.key != "" && c.byKey[entry.key] == id {
		delete(c.byKey, entry.key)
	}
	entry.program.Close()
}
