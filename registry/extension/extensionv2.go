package extension

import (
	c "context"
	"fmt"
	"net/http"

	"github.com/distribution/distribution/v3"
	v2 "github.com/distribution/distribution/v3/registry/api/v2"
	"github.com/distribution/distribution/v3/registry/storage"
)

type RouteDispatchFunc func(extContext *Context, r *http.Request) http.Handler

// Route describes an extended route.
type ExtendedRoute struct {
	Namespace  string
	Extension  string
	Component  string
	Descriptor v2.RouteDescriptor
	Dispatcher RouteDispatchFunc
}

type ExtendedNamespace interface {
	storage.ExtendedStorage
	GetRepositoryRoutes(base distribution.Namespace) []ExtendedRoute
	GetRegistryRoutes(base distribution.Namespace) []ExtendedRoute
}

type InitExtendedNamespace func(ctx c.Context, options map[string]interface{}) (ExtendedNamespace, error)

var extensions map[string]InitExtendedNamespace

// Register is used to register an InitFunc for
// a server extension backend with the given name.
func Register(name string, initFunc InitExtendedNamespace) error {
	if extensions == nil {
		extensions = make(map[string]InitExtendedNamespace)
	}

	if _, exists := extensions[name]; exists {
		return fmt.Errorf("name already registered: %s", name)
	}

	extensions[name] = initFunc

	return nil
}

// Get constructs a server extension with the given options using the named backend.
func Get(ctx c.Context, name string, options map[string]interface{}) (ExtendedNamespace, error) {
	if extensions != nil {
		if initFunc, exists := extensions[name]; exists {
			return initFunc(ctx, options)
		}
	}

	return nil, fmt.Errorf("no server repository extension registered with name: %s", name)
}
