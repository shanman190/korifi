package workloads_test

import (
	korifiv1alpha1 "code.cloudfoundry.org/korifi/controllers/api/v1alpha1"
	. "code.cloudfoundry.org/korifi/controllers/controllers/workloads/testutils"
	"code.cloudfoundry.org/korifi/tools"
	"code.cloudfoundry.org/korifi/tools/image"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
)

var _ = Describe("CFDockerBuildReconciler Integration Tests", func() {
	var (
		cfSpace       *korifiv1alpha1.CFSpace
		cfApp         *korifiv1alpha1.CFApp
		cfPackageGUID string
		cfBuild       *korifiv1alpha1.CFBuild
	)

	BeforeEach(func() {
		imageConfigGetter.ConfigReturns(image.Config{
			Labels:       map[string]string{},
			ExposedPorts: []int32{},
			User:         "1000",
		}, nil)

		cfSpace = createSpace(cfOrg)

		cfApp = &korifiv1alpha1.CFApp{
			ObjectMeta: metav1.ObjectMeta{
				Name:      PrefixedGUID("cf-app"),
				Namespace: cfSpace.Status.GUID,
			},
			Spec: korifiv1alpha1.CFAppSpec{
				DisplayName:  PrefixedGUID("cf-app-display-name"),
				DesiredState: "STOPPED",
				Lifecycle: korifiv1alpha1.Lifecycle{
					Type: "docker",
					Data: korifiv1alpha1.LifecycleData{},
				},
			},
		}
		Expect(adminClient.Create(ctx, cfApp)).To(Succeed())

		cfPackageGUID = PrefixedGUID("cf-package")
		Expect(adminClient.Create(ctx, &korifiv1alpha1.CFPackage{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cfPackageGUID,
				Namespace: cfSpace.Status.GUID,
			},
			Spec: korifiv1alpha1.CFPackageSpec{
				Type: "docker",
				AppRef: corev1.LocalObjectReference{
					Name: cfApp.Name,
				},
				Source: korifiv1alpha1.PackageSource{
					Registry: korifiv1alpha1.Registry{
						Image:            "some/image",
						ImagePullSecrets: []corev1.LocalObjectReference{{Name: "source-image-secret"}},
					},
				},
			},
		})).To(Succeed())

		cfBuild = &korifiv1alpha1.CFBuild{
			ObjectMeta: metav1.ObjectMeta{
				Name:      PrefixedGUID("cf-build"),
				Namespace: cfSpace.Status.GUID,
			},
			Spec: korifiv1alpha1.CFBuildSpec{
				PackageRef: corev1.LocalObjectReference{
					Name: cfPackageGUID,
				},
				AppRef: corev1.LocalObjectReference{
					Name: cfApp.Name,
				},
				Lifecycle: korifiv1alpha1.Lifecycle{
					Type: "docker",
				},
			},
		}
	})

	JustBeforeEach(func() {
		Expect(adminClient.Create(ctx, cfBuild)).To(Succeed())
	})

	It("sets the observed generation in the status", func() {
		Eventually(func(g Gomega) {
			g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfBuild), cfBuild)).To(Succeed())
			g.Expect(cfBuild.Status.ObservedGeneration).To(Equal(cfBuild.Generation))
		}).Should(Succeed())
	})

	It("sets the app as build owner", func() {
		Eventually(func(g Gomega) {
			g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfBuild), cfBuild)).To(Succeed())
			g.Expect(cfBuild.GetOwnerReferences()).To(ConsistOf(
				metav1.OwnerReference{
					APIVersion:         korifiv1alpha1.GroupVersion.Identifier(),
					Kind:               "CFApp",
					Name:               cfApp.Name,
					UID:                cfApp.UID,
					Controller:         tools.PtrTo(true),
					BlockOwnerDeletion: tools.PtrTo(true),
				},
			))
		}).Should(Succeed())
	})

	It("cleans up older builds and droplets", func() {
		Eventually(func(g Gomega) {
			found := false
			for i := 0; i < buildCleaner.CleanCallCount(); i++ {
				_, app := buildCleaner.CleanArgsForCall(i)
				if app.Name == cfApp.Name && app.Namespace == cfSpace.Status.GUID {
					found = true
					break
				}
			}
			g.Expect(found).To(BeTrue())
		}).Should(Succeed())
	})

	It("makes the build succeed", func() {
		Eventually(func(g Gomega) {
			g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfBuild), cfBuild)).To(Succeed())
			g.Expect(meta.IsStatusConditionFalse(cfBuild.Status.Conditions, korifiv1alpha1.StagingConditionType)).To(BeTrue())
			g.Expect(meta.IsStatusConditionTrue(cfBuild.Status.Conditions, korifiv1alpha1.SucceededConditionType)).To(BeTrue())
			g.Expect(cfBuild.Status.Droplet).NotTo(BeNil())
			g.Expect(cfBuild.Status.Droplet.Registry.Image).To(Equal("some/image"))
			g.Expect(cfBuild.Status.Droplet.Registry.ImagePullSecrets).To(ConsistOf(corev1.LocalObjectReference{Name: "source-image-secret"}))
		}).Should(Succeed())
	})

	It("fetches the image config", func() {
		Expect(imageConfigGetter.ConfigCallCount()).NotTo(BeZero())
		_, creds, imageRef := imageConfigGetter.ConfigArgsForCall(imageConfigGetter.ConfigCallCount() - 1)
		Expect(imageRef).To(Equal("some/image"))
		Expect(creds).To(Equal(image.Creds{
			Namespace:   cfSpace.Status.GUID,
			SecretNames: []string{"source-image-secret"},
		}))
	})

	Describe("privileged images", func() {
		succeededCondition := func(g Gomega) metav1.Condition {
			g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfBuild), cfBuild)).To(Succeed())
			g.Expect(meta.IsStatusConditionFalse(cfBuild.Status.Conditions, korifiv1alpha1.StagingConditionType)).To(BeTrue())
			succeedCondition := meta.FindStatusCondition(cfBuild.Status.Conditions, korifiv1alpha1.SucceededConditionType)
			g.Expect(succeedCondition).NotTo(BeNil())

			return *succeedCondition
		}

		haveFailed := gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
			"Status":  Equal(metav1.ConditionFalse),
			"Reason":  Equal("BuildFailed"),
			"Message": ContainSubstring("not supported"),
		})

		When("the user is not specified", func() {
			BeforeEach(func() {
				imageConfigGetter.ConfigReturns(image.Config{}, nil)
			})

			It("fails the build", func() {
				Eventually(succeededCondition).Should(haveFailed)
			})
		})

		When("the user is 'root'", func() {
			BeforeEach(func() {
				imageConfigGetter.ConfigReturns(image.Config{User: "root"}, nil)
			})

			It("fails the build", func() {
				Eventually(succeededCondition).Should(haveFailed)
			})
		})

		When("the user is '0'", func() {
			BeforeEach(func() {
				imageConfigGetter.ConfigReturns(image.Config{User: "0"}, nil)
			})

			It("fails the build", func() {
				Eventually(succeededCondition).Should(haveFailed)
			})
		})

		When("the user is 'root:rootgroup'", func() {
			BeforeEach(func() {
				imageConfigGetter.ConfigReturns(image.Config{User: "root:rootgroup"}, nil)
			})

			It("fails the build", func() {
				Eventually(succeededCondition).Should(haveFailed)
			})
		})

		When("the user is '0:rootgroup'", func() {
			BeforeEach(func() {
				imageConfigGetter.ConfigReturns(image.Config{User: "0:rootgroup"}, nil)
			})

			It("fails the build", func() {
				Eventually(succeededCondition).Should(haveFailed)
			})
		})
	})
})