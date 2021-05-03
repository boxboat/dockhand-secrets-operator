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

package common

import (
	log "github.com/sirupsen/logrus"
	"os"
)

var (
	// Log for global use
	Log = log.New()
)

// ExitIfError will generically handle an error by logging its contents
// and exiting with a return code of 1.
func ExitIfError(err error) {
	if err != nil {
		Log.Errorf("%v", err)
		os.Exit(1)
	}
}

func LogIfError(err error) {
	if err != nil {
		Log.Warnf("%v", err)
	}
}
