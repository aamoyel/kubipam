/*


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

package v1alpha1

const (
	// ConditionTypeReady represents the fact that the reconciliation of
	// the resource has succeeded.
	ConditionTypeReady string = "Ready"

	// ReconciliationSucceededReason represents the fact that reconciliation has succeeded.
	ReconciliationSucceededReason string = "ReconciliationSucceeded"

	// ReconciliationFailedReason represents the fact that reconciliation has failed.
	ReconciliationFailedReason string = "ReconciliationFailed"

	// ProgressingReason represents the fact that the reconciliation of the
	// resource is underway.
	ProgressingReason string = "Progressing"
)
