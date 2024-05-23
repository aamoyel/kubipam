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

			By("Creating a new IPClaim with bad prefix")
			ipClaimName := "test-ip"
			ipclaim := getIPClaim("IP", false, ipClaimName, ipClaimCidr)
			Expect(k8sClient.Create(ctx, ipclaim)).Should(Succeed())

			By("Checking if the 'registered' status field is set to 'true'")
			ipClaimLookupKey := types.NamespacedName{Name: ipClaimName}
			claimedIP := &ipamv1alpha1.IPClaim{}
			Eventually(func() (bool, error) {
				err := k8sClient.Get(ctx, ipClaimLookupKey, claimedIP)
				if err != nil {
					return false, err
				}
				return claimedIP.Status.Registered, nil
			}, timeout, interval).Should(Equal(true))
		})
	})
})

func getIPClaim(claimType string, specific bool, name string, cidrName string) *ipamv1alpha1.IPClaim {
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
