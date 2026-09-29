package authz

// Guard binds how a resource is loaded, how its ownership is projected and
// which rules govern read and write access. Service helpers declare one Guard
// per aggregate and call RequireRead / RequireWrite.
type Guard[T any] struct {
	// Load fetches the resource. Its error is returned verbatim, so the loader
	// is responsible for mapping DB / not-found errors to service error codes.
	Load func(id string) (T, error)
	// Owner projects the loaded resource's ownership.
	Owner func(T) Owned
	// Read is checked by RequireRead and first by RequireWrite.
	Read Rule
	// Write is checked by RequireWrite after Read passed. A zero Write rule
	// means "same as Read".
	Write Rule
	// ClaimsFirst rejects nil / role-disallowed claims before Load runs (so an
	// anonymous caller never learns whether the id exists). When false the
	// resource is loaded first, preserving helpers whose load error wins.
	ClaimsFirst bool
	// Missing reports a loaded-but-absent resource (e.g. nil pointer / zero
	// id). Such a resource is denied with the Read rule's error so existence
	// is not leaked.
	Missing func(T) bool
}

func (g Guard[T]) writeRule() Rule {
	if isZeroRule(g.Write) {
		return g.Read
	}
	return g.Write
}

func isZeroRule(r Rule) bool {
	return r.Roles == nil && r.Scope == nil && !r.OwnerOnly && !r.AllowShared &&
		!r.AllowPublic && r.Code == 0 && r.Message == ""
}

func (g Guard[T]) load(id string, c *Claims, pre Rule) (T, error) {
	var zero T
	if g.ClaimsFirst {
		if err := pre.RequireClaims(c); err != nil {
			return zero, err
		}
	}
	res, err := g.Load(id)
	if err != nil {
		return zero, err
	}
	if g.Missing != nil && g.Missing(res) {
		return zero, g.Read.Deny()
	}
	return res, nil
}

// RequireRead loads id and checks the Read rule.
func (g Guard[T]) RequireRead(id string, c *Claims) (T, error) {
	res, err := g.load(id, c, g.Read)
	if err != nil {
		return res, err
	}
	if err := g.CheckRead(res, c); err != nil {
		var zero T
		return zero, err
	}
	return res, nil
}

// RequireWrite loads id and checks the Read rule then the Write rule.
func (g Guard[T]) RequireWrite(id string, c *Claims) (T, error) {
	res, err := g.load(id, c, g.Read)
	if err != nil {
		return res, err
	}
	if err := g.CheckWrite(res, c); err != nil {
		var zero T
		return zero, err
	}
	return res, nil
}

// CheckRead applies the Read rule to an already loaded resource.
func (g Guard[T]) CheckRead(res T, c *Claims) error {
	return g.Read.Check(c, g.Owner(res))
}

// CheckWrite applies Read then Write to an already loaded resource.
func (g Guard[T]) CheckWrite(res T, c *Claims) error {
	if err := g.Read.Check(c, g.Owner(res)); err != nil {
		return err
	}
	return g.writeRule().Check(c, g.Owner(res))
}

// RequireRead is the free-function form for resources that implement
// TenantOwned directly.
func RequireRead[T TenantOwned](c *Claims, res T, rule Rule) (T, error) {
	if err := rule.Check(c, res); err != nil {
		var zero T
		return zero, err
	}
	return res, nil
}

// RequireWrite checks read then write rules against a TenantOwned resource.
func RequireWrite[T TenantOwned](c *Claims, res T, read, write Rule) (T, error) {
	var zero T
	if err := read.Check(c, res); err != nil {
		return zero, err
	}
	if err := write.Check(c, res); err != nil {
		return zero, err
	}
	return res, nil
}
