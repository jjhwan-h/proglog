package auth

import (
	"fmt"

	"github.com/casbin/casbin/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Authorizer struct {
	enforcer *casbin.Enforcer
}

func New(model, policy string) *Authorizer {
	enforcer, err := casbin.NewEnforcer(model, policy)
	if err != nil {
		panic(err)
	}
	return &Authorizer{
		enforcer: enforcer,
	}
}

func (a *Authorizer) Authorize(subject, object, action string) error {
	// subject가 object에 action을 할 수 있는지?
	res, err := a.enforcer.Enforce(subject, object, action)
	if err != nil {
		return err
	}

	if !res {
		msg := fmt.Sprintf(
			"%s not permitted to %s to %s",
			subject,
			action,
			object,
		)
		st := status.New(codes.PermissionDenied, msg)
		return st.Err()
	}
	return nil
}
