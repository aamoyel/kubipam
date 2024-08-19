/*
Copyright 2024.

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

package controllers

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	ipamv1alpha1 "github.com/aamoyel/kubipam/api/v1alpha1"
)

var _ = Describe("IPClaim controller", func() {
	Context("When create a claim for a non specific IP", func() {
		It("Should change 'registered' field in status and get an ip from the ipam", func() {
			ipClaimCidr := "ipclaim-cidr"
			ctx := context.Background()

			By("Creating a new IPCidr")
			ipcidr := getIpCidr(ipClaimCidr, "172.16.0.0/16")
			Expect(k8sClient.Create(ctx, ipcidr)).Should(Succeed())

			By("Creating a new IPClaim to claim an IP addr")
			ipClaimName := "test-ip"
			ipclaim := getIPClaim("IP", false, ipClaimName, ipClaimCidr)
			Expect(k8sClient.Create(ctx, ipclaim)).Should(Succeed())

			By("Checking if the 'registered' status field is set to 'true'")
			ipClaimLookupKey := types.NamespacedName{Name: ipClaimName}
			ipClaimCR := &ipamv1alpha1.IPClaim{}
			Eventually(func() (bool, error) {
				err := k8sClient.Get(ctx, ipClaimLookupKey, ipClaimCR)
				if err != nil {
					return false, err
				}
				return ipClaimCR.Status.Registered, nil
			}, timeout, interval).Should(Equal(true))
		})
	})
})

func getIPClaim(
	claimType string,
	specific bool,
	name string,
	cidrName string,
) *ipamv1alpha1.IPClaim {
	claim := &ipamv1alpha1.IPClaim{}
	if claimType == "IP" {
		if !specific {
			claim = &ipamv1alpha1.IPClaim{
				TypeMeta: metav1.TypeMeta{
					APIVersion: ipamv1alpha1.GroupVersion.Group + "/" + ipamv1alpha1.GroupVersion.Version,
					Kind:       "IPClaim",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Spec: ipamv1alpha1.IPClaimSpec{
					Type: claimType,
					IPCidrRef: &ipamv1alpha1.IPCidrRefSpec{
						Name: cidrName,
					},
				},
			}
		}
	}

	return claim
}
