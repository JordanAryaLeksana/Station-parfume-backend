package services

import (
	"backend/src/repository"
	"errors"
	"testing"
)

func TestRoleRules(t *testing.T) {
	cases := []struct {
		caller, role string
		want         error
	}{
		{repository.RoleAdmin, repository.RoleUser, nil},
		{repository.RoleOwner, repository.RoleUser, nil},
		{repository.RoleOwner, repository.RoleWarehouse, nil},
		{repository.RoleOwner, repository.RoleAdmin, nil},
		{repository.RoleAdmin, repository.RoleAdmin, ErrForbidden}, // admin tidak boleh mengelola staf
		{repository.RoleAdmin, repository.RoleOwner, ErrForbidden}, // celah lama: admin → owner
		{repository.RoleOwner, repository.RoleOwner, ErrForbidden}, // owner tidak dikelola lewat /users
		{repository.RoleAdmin, repository.RoleCS, ErrForbidden},
		{repository.RoleOwner, "superadmin", ErrValidation},
	}
	for _, c := range cases {
		err := checkRole(c.caller, c.role)
		if (c.want == nil) != (err == nil) || (c.want != nil && !errors.Is(err, c.want)) {
			t.Errorf("checkRole(%s, %s) = %v, want %v", c.caller, c.role, err, c.want)
		}
	}
}
