package controller

import (
	"context"
	"fmt"
	controllerv1 "github.com/Saleh7127/sample-controller/pkg/apis/saleh.dev/v1alpha1"
	clientset "github.com/Saleh7127/sample-controller/pkg/client/clientset/versioned"
	informer "github.com/Saleh7127/sample-controller/pkg/client/informers/externalversions/saleh.dev/v1alpha1"
	lister "github.com/Saleh7127/sample-controller/pkg/client/listers/saleh.dev/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsinformer "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/kubernetes"
	appslisters "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"log"
	"time"
)

// Controller is the controller implementation for Uban resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// sampleclientset is a clientset for our own API group
	sampleclientset clientset.Interface

	deploymentsLister appslisters.DeploymentLister
	deploymentsSynced cache.InformerSynced
	ubanLister        lister.UbanLister
	ubanSynced        cache.InformerSynced

	// work-queue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workQueue workqueue.RateLimitingInterface
}

// NewController returns a new sample controller
func NewController(
	kubeClient kubernetes.Interface,
	oooClient clientset.Interface,
	deploymentInformer appsinformer.DeploymentInformer,
	ubanInformer informer.UbanInformer) *Controller {

	ctrl := &Controller{
		kubeclientset:     kubeClient,
		sampleclientset:   oooClient,
		deploymentsLister: deploymentInformer.Lister(),
		deploymentsSynced: deploymentInformer.Informer().HasSynced,
		ubanLister:        ubanInformer.Lister(),
		ubanSynced:        ubanInformer.Informer().HasSynced,
		workQueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Uban"),
	}
	log.Println("Setting up event Handlers")

	// Set up an event handler for when Uban resources change
	_, err := ubanInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: ctrl.enqueueUban,
		UpdateFunc: func(oldObj, newObj interface{}) {
			ctrl.enqueueUban(newObj)
		},
		DeleteFunc: func(obj interface{}) {
			ctrl.enqueueUban(obj)
		},
	})
	if err != nil {
		log.Println("Got an error setting up event handlers")
	}
	// what if deployment resources changes??
	return ctrl
}

// enqueueUban takes an Uban resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Uban and only can call by a
// Controller object

func (c *Controller) enqueueUban(obj interface{}) {
	log.Println("Enqueueing Uban ....")
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workQueue.AddRateLimited(key)
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shut down the work-queue and wait for
// workers to finish processing their current work items.

func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workQueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	log.Println("Starting Uban Controller")

	// Wait for the caches to be synced before starting workers
	log.Println("Waiting for informer caches to sync")

	if ok := cache.WaitForCacheSync(stopCh, c.deploymentsSynced, c.ubanSynced); !ok {
		return fmt.Errorf("_Failed to wait for cache to sync")
	}
	log.Println("Staring Workers")

	// Launch two workers to process Uban resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	log.Println("Working Started")
	<-stopCh
	log.Println("Shutting Down Workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the work-queue.
func (c *Controller) runWorker() {
	for c.ProcessNextItem() {
	}
}

func (c *Controller) ProcessNextItem() bool {
	obj, shutdown := c.workQueue.Get()

	if shutdown {
		return false
	}
	// We wrap this block in a func, so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the work-queue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the work-queue and attempted again after a back-off period.

		defer c.workQueue.Done(obj)
		var key string
		var ok bool

		// We expect strings to come off the work-queue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// work-queue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the work-queue.

		if key, ok = obj.(string); !ok {
			// As the item in the work-queue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.

			c.workQueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("_Expected string in work-queue but got %#v", obj))

			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Uban resource to be synced.
		if err := c.syncHandler(key); err != nil {
			// Put the item back on the work-queue to handle any transient errors.
			c.workQueue.AddRateLimited(key)
			return fmt.Errorf("_Error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item, so it does not
		// get queued again until another change happens.
		c.workQueue.Forget(obj)
		log.Printf("successfully synced '%s'\n", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}
	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Uban resource
// with the current status of the resource.
// implement the business logic here.

func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("_Invalid resource key : %s", key))
	}

	// Get the Uban resource with this namespace/name
	uban, err := c.ubanLister.Ubans(namespace).Get(name)
	if err != nil {
		//The Uban resource may no longer exit, in which case we stop processing.
		if errors.IsNotFound(err) {
			// We choose to absorb the error here as the worker would requeue the
			// resource otherwise. Instead, the next time the resource is updated
			// the resource will be queued again.
			utilruntime.HandleError(fmt.Errorf("_Uban '%s' in work queue no longer exists", key))
			return nil
		}
		return err
	}

	deploymentName := uban.Name + "-" + uban.Spec.Name
	if uban.Spec.Name == "" {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated
		// the resource will be queued again.
		deploymentName = uban.Name + "-" + "random-name"
	}

	// Get the deployment with the name specified in Uban.spec
	deployment, err := c.deploymentsLister.Deployments(namespace).Get(deploymentName)

	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		deployment, err = c.kubeclientset.AppsV1().Deployments(uban.Namespace).Create(context.TODO(), newDeployment(uban, deploymentName), metav1.CreateOptions{})
	}

	// If an error occurs during Get/Create, we'll requeue the item, so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	// If this number of the replicas on the Uban resource is specified, and the
	// number does not equal the current desired replicas on the Deployment, we
	// should update the Deployment resource.

	if uban.Spec.Replicas != nil && *uban.Spec.Replicas != *deployment.Spec.Replicas {
		log.Printf("Uban %s replicas: %d, deployment replicas: %d\n", name, *uban.Spec.Replicas, *deployment.Spec.Replicas)

		deployment, err = c.kubeclientset.AppsV1().Deployments(namespace).Update(context.TODO(), newDeployment(uban, deploymentName), metav1.UpdateOptions{})
		// If an error occurs during Update, we'll requeue the item, so we can
		// attempt processing again later. This could have been caused by a
		// temporary network failure, or any other transient reason.
		if err != nil {
			return err
		}
	}

	// Finally, we update the status block of the Uban resource to reflect the
	// current state of the world

	err = c.updateUbanStatus(uban, deployment)
	if err != nil {
		return err
	}

	serviceName := deploymentName + "-service"
	//check is service already exist or not
	service, err := c.kubeclientset.CoreV1().Services(uban.Namespace).Get(context.TODO(), serviceName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		service, err = c.kubeclientset.CoreV1().Services(uban.Namespace).Create(context.TODO(), newService(uban, serviceName), metav1.CreateOptions{})
		if err != nil {
			log.Println(err)
			return err
		}
		log.Printf("\nService %s created .....\n", service.Name)
	} else if err != nil {
		log.Println(err)
		return err
	}

	_, err = c.kubeclientset.CoreV1().Services(uban.Namespace).Update(context.TODO(), service, metav1.UpdateOptions{})
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func (c *Controller) updateUbanStatus(uban *controllerv1.Uban, deployment *appsv1.Deployment) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance

	ubanCopy := uban.DeepCopy()
	ubanCopy.Status.AvailableReplicas = deployment.Status.AvailableReplicas

	// If the CustomResourceSubresources feature gate is not enabled,
	// we must use Update instead of UpdateStatus to update the Status block of the Uban resource.
	// UpdateStatus will not allow changes to the Spec of the resource,
	// which is ideal for ensuring nothing other than resource status has been updated.
	_, err := c.sampleclientset.SalehV1alpha1().Ubans(uban.Namespace).Update(context.TODO(), ubanCopy, metav1.UpdateOptions{})
	return err
}
