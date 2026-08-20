package tenant

import (
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Sentinel errors returned by Store implementations.
var (
	// ErrNotFound is returned when a tenant or workspace does not exist.
	ErrNotFound = errors.New("tenant: not found")
	// ErrDuplicateSlug is returned when a slug is already taken.
	ErrDuplicateSlug = errors.New("tenant: slug already exists")
)

// Tenant is a top-level customer of the platform. Workspaces belong to a
// Tenant and follow the <company>.workspace.sapliy.com model.
type Tenant struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	Plan      string    `json:"plan"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Workspace is a scoped operational surface within a Tenant.
type Workspace struct {
	ID       string `json:"id"`
	TenantID string `json:"tenantId"`
	Name     string `json:"name"`
	Slug     string `json:"slug"`
}

// Store is the persistence contract for tenants and workspaces.
// DB-backed implementations are a later phase; the MVP ships an in-memory
// implementation (NewMemoryStore).
type Store interface {
	CreateTenant(t *Tenant) error
	GetTenant(id string) (*Tenant, error)
	ListWorkspaces(tenantID string) ([]*Workspace, error)
	CreateWorkspace(w *Workspace) error
}

// MemoryStore is a concurrency-safe, in-memory Store for the MVP.
type MemoryStore struct {
	mu         sync.RWMutex
	tenants    map[string]*Tenant
	tenantSlug map[string]string
	workspaces map[string][]*Workspace
	wsSlug     map[string]map[string]bool
}

// NewMemoryStore creates an empty in-memory Store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		tenants:    make(map[string]*Tenant),
		tenantSlug: make(map[string]string),
		workspaces: make(map[string][]*Workspace),
		wsSlug:     make(map[string]map[string]bool),
	}
}

// CreateTenant stores a new tenant. Tenant slugs are globally unique. An
// empty ID is assigned a UUID and empty timestamps default to now (UTC).
func (s *MemoryStore) CreateTenant(t *Tenant) error {
	if t == nil {
		return fmt.Errorf("tenant: nil tenant")
	}
	if strings.TrimSpace(t.Name) == "" {
		return fmt.Errorf("tenant: name is required")
	}
	if strings.TrimSpace(t.Slug) == "" {
		return fmt.Errorf("tenant: slug is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.tenantSlug[t.Slug]; ok {
		return fmt.Errorf("%w: %s", ErrDuplicateSlug, t.Slug)
	}

	if t.ID == "" {
		t.ID = newID()
	}
	now := time.Now().UTC()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	if t.UpdatedAt.IsZero() {
		t.UpdatedAt = now
	}

	s.tenants[t.ID] = cloneTenant(t)
	s.tenantSlug[t.Slug] = t.ID
	s.workspaces[t.ID] = []*Workspace{}
	return nil
}

// GetTenant returns a defensive copy of the tenant with the given ID.
func (s *MemoryStore) GetTenant(id string) (*Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.tenants[id]
	if !ok {
		return nil, fmt.Errorf("%w: tenant %s", ErrNotFound, id)
	}
	return cloneTenant(t), nil
}

// ListWorkspaces returns a defensive copy of the workspaces for a tenant.
func (s *MemoryStore) ListWorkspaces(tenantID string) ([]*Workspace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.tenants[tenantID]; !ok {
		return nil, fmt.Errorf("%w: tenant %s", ErrNotFound, tenantID)
	}
	ws := s.workspaces[tenantID]
	out := make([]*Workspace, len(ws))
	for i, w := range ws {
		out[i] = cloneWorkspace(w)
	}
	return out, nil
}

// CreateWorkspace stores a new workspace for an existing tenant. Workspace
// slugs are unique within their tenant.
func (s *MemoryStore) CreateWorkspace(w *Workspace) error {
	if w == nil {
		return fmt.Errorf("tenant: nil workspace")
	}
	if strings.TrimSpace(w.Name) == "" {
		return fmt.Errorf("tenant: workspace name is required")
	}
	if strings.TrimSpace(w.Slug) == "" {
		return fmt.Errorf("tenant: workspace slug is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.tenants[w.TenantID]; !ok {
		return fmt.Errorf("%w: tenant %s", ErrNotFound, w.TenantID)
	}
	if s.wsSlug[w.TenantID][w.Slug] {
		return fmt.Errorf("%w: workspace %s in tenant %s", ErrDuplicateSlug, w.Slug, w.TenantID)
	}

	if w.ID == "" {
		w.ID = newID()
	}
	if s.wsSlug[w.TenantID] == nil {
		s.wsSlug[w.TenantID] = make(map[string]bool)
	}
	s.wsSlug[w.TenantID][w.Slug] = true
	s.workspaces[w.TenantID] = append(s.workspaces[w.TenantID], cloneWorkspace(w))
	return nil
}

// cloneTenant returns a defensive copy of a Tenant.
func cloneTenant(t *Tenant) *Tenant {
	c := *t
	return &c
}

// cloneWorkspace returns a defensive copy of a Workspace.
func cloneWorkspace(w *Workspace) *Workspace {
	c := *w
	return &c
}

// newID returns a random RFC 4122 version 4 UUID string.
// It is stdlib-only to keep the tenant package dependency-free.
func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Sprintf("tenant: failed to read random bytes: %v", err))
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
