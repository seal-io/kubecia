package apis

import (
	"path"
)

func RoutePrefix(namespace string) string {
	return path.Join("/", namespace) + "/"
}

func Route(namespace string, paths ...string) string {
	ps := make([]string, 0, len(paths)+3)
	ps = append(ps, "http", ".", namespace)
	ps = append(ps, paths...)

	return path.Join(ps...)
}
