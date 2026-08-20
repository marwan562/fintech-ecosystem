package tenant_test

import (
	"errors"
	"testing"

	"github.com/sapliy/sapliy-core/internal/tenant"
)

func TestMemoryStore_CreateAndGetTenant(t *testing.T) {
	store := tenant.NewMemoryStore()

	tnt := &tenant.Tenant{Name: "Acme Corp", Slug: "acme", Plan: "enterprise"}
	if err := store.CreateTenant(tnt); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if tnt.ID == "" {
		t.Fatal("CreateTenant should assign an ID")
	}

	got, err := store.GetTenant(tnt.ID)
	if err != nil {
		t.Fatalf("GetTenant: %v", err)
	}
	if got.Name != "Acme Corp" {
		t.Errorf("Name = %q, want Acme Corp", got.Name)
	}
	if got.Slug != "acme" {
		t.Errorf("Slug = %q, want acme", got.Slug)
	}
	if got.Plan != "enterprise" {
		t.Errorf("Plan = %q, want enterprise", got.Plan)
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func TestMemoryStore_GetTenantNotFound(t *testing.T) {
	store := tenant.NewMemoryStore()
	_, err := store.GetTenant("nope")
	if !errors.Is(err, tenant.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestMemoryStore_CreateTenant_DuplicateSlug(t *testing.T) {
	store := tenant.NewMemoryStore()

	if err := store.CreateTenant(&tenant.Tenant{Name: "Acme", Slug: "acme"}); err != nil {
		t.Fatalf("first CreateTenant: %v", err)
	}
	err := store.CreateTenant(&tenant.Tenant{Name: "Acme Again", Slug: "acme"})
	if !errors.Is(err, tenant.ErrDuplicateSlug) {
		t.Fatalf("err = %v, want ErrDuplicateSlug", err)
	}
}

func TestMemoryStore_CreateTenant_MissingNameOrSlug(t *testing.T) {
	store := tenant.NewMemoryStore()

	if err := store.CreateTenant(&tenant.Tenant{Name: "", Slug: "acme"}); err == nil {
		t.Error("CreateTenant with empty name should fail")
	}
	if err := store.CreateTenant(&tenant.Tenant{Name: "Acme", Slug: ""}); err == nil {
		t.Error("CreateTenant with empty slug should fail")
	}
}

func TestMemoryStore_Workspaces(t *testing.T) {
	store := tenant.NewMemoryStore()

	tnt := &tenant.Tenant{Name: "Acme", Slug: "acme"}
	if err := store.CreateTenant(tnt); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	ws := &tenant.Workspace{TenantID: tnt.ID, Name: "Production", Slug: "prod"}
	if err := store.CreateWorkspace(ws); err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}
	if ws.ID == "" {
		t.Fatal("CreateWorkspace should assign an ID")
	}

	ws2 := &tenant.Workspace{TenantID: tnt.ID, Name: "Staging", Slug: "staging"}
	if err := store.CreateWorkspace(ws2); err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}

	list, err := store.ListWorkspaces(tnt.ID)
	if err != nil {
		t.Fatalf("ListWorkspaces: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("len(list) = %d, want 2", len(list))
	}
	if list[0].Slug != "prod" || list[1].Slug != "staging" {
		t.Errorf("workspace slugs = %q, %q", list[0].Slug, list[1].Slug)
	}

	// Mutating the returned list must not affect the store.
	list[0].Name = "Hacked"
	again, err := store.ListWorkspaces(tnt.ID)
	if err != nil {
		t.Fatalf("ListWorkspaces: %v", err)
	}
	if again[0].Name == "Hacked" {
		t.Error("ListWorkspaces should return defensive copies")
	}
}

func TestMemoryStore_CreateWorkspace_DuplicateSlug(t *testing.T) {
	store := tenant.NewMemoryStore()

	tnt := &tenant.Tenant{Name: "Acme", Slug: "acme"}
	if err := store.CreateTenant(tnt); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if err := store.CreateWorkspace(&tenant.Workspace{TenantID: tnt.ID, Name: "Prod", Slug: "prod"}); err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}

	err := store.CreateWorkspace(&tenant.Workspace{TenantID: tnt.ID, Name: "Prod Two", Slug: "prod"})
	if !errors.Is(err, tenant.ErrDuplicateSlug) {
		t.Fatalf("err = %v, want ErrDuplicateSlug", err)
	}

	// The same slug is allowed for a different tenant.
	other := &tenant.Tenant{Name: "Other", Slug: "other"}
	if err := store.CreateTenant(other); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if err := store.CreateWorkspace(&tenant.Workspace{TenantID: other.ID, Name: "Prod", Slug: "prod"}); err != nil {
		t.Fatalf("same workspace slug under another tenant should be allowed: %v", err)
	}
}

func TestMemoryStore_CreateWorkspace_UnknownTenant(t *testing.T) {
	store := tenant.NewMemoryStore()
	err := store.CreateWorkspace(&tenant.Workspace{TenantID: "ghost", Name: "Prod", Slug: "prod"})
	if !errors.Is(err, tenant.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestMemoryStore_ListWorkspaces_UnknownTenant(t *testing.T) {
	store := tenant.NewMemoryStore()
	_, err := store.ListWorkspaces("ghost")
	if !errors.Is(err, tenant.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
