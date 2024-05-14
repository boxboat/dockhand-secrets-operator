/*
Copyright © 2021 BoxBoat

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	dockhand "github.com/boxboat/dockhand-secrets-operator/pkg/apis/dhs.dockhand.dev/v1alpha2"
	controllergen "github.com/rancher/wrangler/v2/pkg/controller-gen"
	"github.com/rancher/wrangler/v2/pkg/controller-gen/args"
	// Ensure gvk gets loaded in wrangler/pkg/gvk cache
	_ "github.com/rancher/wrangler/v2/pkg/generated/controllers/apiextensions.k8s.io/v1"
)

func main() {
	controllergen.Run(args.Options{
		OutputPackage: "github.com/boxboat/dockhand-secrets-operator/pkg/generated",
		Boilerplate:   "scripts/boilerplate.go.txt",
		Groups: map[string]args.Group{
			"dhs.dockhand.dev": {
				Types: []interface{}{
					dockhand.Secret{},
					dockhand.Profile{},
				},
				GenerateTypes: true,
			},
		},
	})
}
