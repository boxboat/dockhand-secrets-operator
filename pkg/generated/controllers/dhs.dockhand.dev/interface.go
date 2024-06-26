/*
Copyright © 2024 BoxBoat

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
// Code generated by main. DO NOT EDIT.

package dhs

import (
	v1alpha2 "github.com/boxboat/dockhand-secrets-operator/pkg/generated/controllers/dhs.dockhand.dev/v1alpha2"
	"github.com/rancher/lasso/pkg/controller"
)

type Interface interface {
	V1alpha2() v1alpha2.Interface
}

type group struct {
	controllerFactory controller.SharedControllerFactory
}

// New returns a new Interface.
func New(controllerFactory controller.SharedControllerFactory) Interface {
	return &group{
		controllerFactory: controllerFactory,
	}
}

func (g *group) V1alpha2() v1alpha2.Interface {
	return v1alpha2.New(g.controllerFactory)
}
