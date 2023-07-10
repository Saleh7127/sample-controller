package main

import (
	"flag"
	clientset "github.com/Saleh7127/sample-controller/pkg/client/clientset/versioned"
	informers "github.com/Saleh7127/sample-controller/pkg/client/informers/externalversions"
	"github.com/Saleh7127/sample-controller/pkg/controller"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	_ "k8s.io/code-generator"
	"log"
	"path/filepath"
	"time"
)

func main() {

	log.Println("Configure KubeConfig............")

	var kubeConfig *string

	if home := homedir.HomeDir(); home != "" {
		kubeConfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeConfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}

	config, err := clientcmd.BuildConfigFromFlags("", *kubeConfig)
	if err != nil {
		panic(err)
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	oooClient, err := clientset.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	// Initialise the informer resource and here we will be using shared-informer factory instead of simple informers
	// because in case if we need to query / watch multiple Group versions, and it’s a good practise as well
	// NewSharedInformerFactory will create a new ShareInformerFactory for "all namespaces"
	// 30*time.Second is the re-sync period to update the in-memory cache of informer //

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	selfInformerFactory := informers.NewSharedInformerFactory(oooClient, time.Second*30)

	// From this informer-factory we can create specific informers for every group version resource
	// that are default available in k8s environment such as Pods, deployment, etc
	// podInformer := kubeInformationFactory.Core().V1().Pods()

	ctrl := controller.NewController(kubeClient, oooClient, kubeInformerFactory.Apps().V1().Deployments(), selfInformerFactory.Saleh().V1alpha1().Ubans())

	// creating a unbuffered channel to synchronize the update
	stopCh := make(chan struct{})
	kubeInformerFactory.Start(stopCh)
	selfInformerFactory.Start(stopCh)

	if err = ctrl.Run(2, stopCh); err != nil {
		log.Println("Error during running controller")
	}
}
