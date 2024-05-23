package controllers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	ipamv1alpha1 "github.com/aamoyel/kubipam/api/v1alpha1"
)

const (
	timeout       = time.Second * 4
	interval      = time.Second * 1
	validCidrName = "test-cidr"
)

var _ = Describe("IPCidr controller", func() {

	Context("When create IPCidr", func() {
		It("Should change 'registered' field in status and create the cidr in the ipam", func() {
			ipCidrLookupKey := types.NamespacedName{Name: validCidrName}
			createdIpCidr := &ipamv1alpha1.IPCidr{}

			By("Creating a new IPCidr")
			ctx := context.Background()
			ipcidr := getIpCidr(validCidrName, "192.168.0.0/16")
			Expect(k8sClient.Create(ctx, ipcidr)).Should(Succeed())

			By("Checking if the 'registered' status field is set to 'true'")
			Eventually(func() (bool, error) {
				err := k8sClient.Get(ctx, ipCidrLookupKey, createdIpCidr)
				if err != nil {
					return false, err
				}
				return createdIpCidr.Status.Registered, nil
			}, timeout, interval).Should(Equal(true))

			By("Checking if the cidr is correctly created in the ipam")
			_, err := ipamer.PrefixFrom(ctx, ipcidr.Spec.Cidr)
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("Should not create the cidr in the ipam and set 'registered' field in status if the cidr is not correctly set of conform", func() {
			By("Creating a new IPCidr with bad prefix")
			ipCidrName := "bad-prefix"
			ctx := context.Background()
			ipcidr := getIpCidr(ipCidrName, "10.10.10.0/33")
			Expect(k8sClient.Create(ctx, ipcidr)).Should(Succeed())

			By("Checking if the 'registered' status field is set to 'false' for the IPCidr that has a bad prefix")
			ipCidrLookupKey := types.NamespacedName{Name: ipCidrName}
			createdIpCidr := &ipamv1alpha1.IPCidr{}
			Eventually(func() (bool, error) {
				err := k8sClient.Get(ctx, ipCidrLookupKey, createdIpCidr)
				if err != nil {
					return false, err
				}
				return createdIpCidr.Status.Registered, nil
			}, timeout, interval).Should(Equal(false))

			By("Creating a new IPCidr that overlap a created one")
			ipCidrName = "overlap-cidr"
			ipcidr = getIpCidr(ipCidrName, "192.168.1.0/24")
			Expect(k8sClient.Create(ctx, ipcidr)).Should(Succeed())

			By("Checking if the 'registered' status field is set to 'false' for the IPCidr that overlap an existing registered cidr")
			ipCidrLookupKey = types.NamespacedName{Name: ipCidrName}
			createdIpCidr = &ipamv1alpha1.IPCidr{}
			Eventually(func() (bool, error) {
				err := k8sClient.Get(ctx, ipCidrLookupKey, createdIpCidr)
				if err != nil {
					return false, err
				}
				return createdIpCidr.Status.Registered, nil
			}, timeout, interval).Should(Equal(false))
		})
	})
	Context("When delete IPCidr", func() {
		It("Should remove the registered cidr from the ipam", func() {
			ipCidrLookupKey := types.NamespacedName{Name: validCidrName}
			createdIpCidr := &ipamv1alpha1.IPCidr{}

			By("Removing the resource from the cluster")
			Eventually(func() error {
				err := k8sClient.Get(ctx, ipCidrLookupKey, createdIpCidr)
				if err != nil {
					return err
				}
				err = k8sClient.Delete(ctx, createdIpCidr)
				if err != nil {
					return err
				}
				return nil
			}, timeout, interval).Should(BeNil())

			By("Checking if the cidr is removed from the ipam")
			Eventually(func() error {
				_, err := ipamer.PrefixFrom(ctx, createdIpCidr.Spec.Cidr)
				if err != nil {
					return err
				}
				return nil
			}, timeout, interval).Should(MatchError(ContainSubstring("NotFound prefix")))
		})
	})
})

func getIpCidr(name string, cidr string) *ipamv1alpha1.IPCidr {
	return &ipamv1alpha1.IPCidr{
		TypeMeta: metav1.TypeMeta{
			APIVersion: ipamv1alpha1.GroupVersion.Group + "/" + ipamv1alpha1.GroupVersion.Version,
			Kind:       "IPCidr",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: ipamv1alpha1.IPCidrSpec{
			Cidr: cidr,
		},
	}
}
