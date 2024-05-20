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

var _ = Describe("IPCidr controller", func() {

	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)
	Context("When create IPCidr", func() {
		It("Should change 'registered' field in status and create the cidr in the ipam", func() {
			ipCidrName := "good-cidr"
			By("Creating a new IPCidr")
			ctx := context.Background()
			ipcidr := getIpCidr(ipCidrName, "192.168.0.0/16")
			Expect(k8sClient.Create(ctx, ipcidr)).Should(Succeed())

			ipCidrLookupKey := types.NamespacedName{Name: ipCidrName}
			createdIpCidr := &ipamv1alpha1.IPCidr{}

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
			ipCidrName := "bad-cidr"
			By("Creating a new IPCidr")
			ctx := context.Background()
			ipcidr := getIpCidr(ipCidrName, "172.16.0.0/33")
			Expect(k8sClient.Create(ctx, ipcidr)).Should(Succeed())
			ipCidrLookupKey := types.NamespacedName{Name: ipCidrName}
			createdIpCidr := &ipamv1alpha1.IPCidr{}

			By("Checking if the 'registered' status field is set to 'false'")
			Eventually(func() (bool, error) {
				err := k8sClient.Get(ctx, ipCidrLookupKey, createdIpCidr)
				if err != nil {
					return false, err
				}
				return createdIpCidr.Status.Registered, nil
			}, timeout, interval).Should(Equal(false))

			// TODO: check overlapping with By (new cidr that overlapp "good-cidr")
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
