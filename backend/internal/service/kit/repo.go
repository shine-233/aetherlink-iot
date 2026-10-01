package kit

import utils "aetherlink-iot/backend/pkg/utils"

// TenantRepo wraps a resource whose DAL getter already filters by tenant:
// Get(id, tenantID). It folds the gate → get → not-found triplet that the
// tenant CRUD services repeat in every Get/Update/Delete.
type TenantRepo[T any] struct {
	Get      func(id, tenantID string) (T, error)
	Gate     Gate
	NotFound NotFound
	// TenantOf picks the tenant passed to the DAL; default c.TenantID.
	TenantOf func(*utils.UserClaims) string
}

func (r TenantRepo[T]) tenant(c *utils.UserClaims) string {
	if r.TenantOf != nil {
		return r.TenantOf(c)
	}
	return c.TenantID
}

// Load runs the gate, fetches id in the caller's tenant and maps load errors.
func (r TenantRepo[T]) Load(c *utils.UserClaims, id string) (T, error) {
	var zero T
	if err := r.Gate.Require(c); err != nil {
		return zero, err
	}
	rec, err := r.Get(id, r.tenant(c))
	if err != nil {
		return zero, r.NotFound.Map(err)
	}
	return rec, nil
}

// MustExist is Load without the record (delete pre-check).
func (r TenantRepo[T]) MustExist(c *utils.UserClaims, id string) error {
	_, err := r.Load(c, id)
	return err
}

// Delete checks existence then calls del(id, tenant). onErr maps a del
// failure; nil returns it verbatim (the historical contract of most
// tenant services, whose delete passed the DAL error through).
func (r TenantRepo[T]) Delete(c *utils.UserClaims, id string, del func(id, tenantID string) error, onErr func(error) error) error {
	if err := r.MustExist(c, id); err != nil {
		return err
	}
	if err := del(id, r.tenant(c)); err != nil {
		if onErr != nil {
			return onErr(err)
		}
		return err
	}
	return nil
}

// List runs the gate, calls list(req, tenant) and returns the
// {"total","list"} map. onErr maps a list failure; nil returns it verbatim.
// It is a free function because Go methods cannot add type parameters.
func List[T, R, E any](r TenantRepo[T], c *utils.UserClaims, req R, list func(R, string) (int64, []E, error), onErr func(error) error) (map[string]interface{}, error) {
	if err := r.Gate.Require(c); err != nil {
		return nil, err
	}
	total, items, err := list(req, r.tenant(c))
	if err != nil {
		if onErr != nil {
			return nil, onErr(err)
		}
		return nil, err
	}
	return ListMap(total, items), nil
}
