package main

import (
	"log"

	apicorev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	informercorev1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	listercorev1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

const (
	secretSyncType            = "k8s.ziwon.dev/secretsync"
	secretSyncSourceNamespace = "secretsync"
)

var namespaceBlacklist = map[string]bool{
	"kube-public":             true,
	"kube-system":             true,
	secretSyncSourceNamespace: true,
}

// K&SController defines an example controller struct for watching the changes with Secrets
type K8SController struct {
	secretGetter          corev1.SecretsGetter
	secretLister          listercorev1.SecretLister
	secretListerSynced    cache.InformerSynced
	namespaceGetter       corev1.NamespacesGetter
	namespaceLister       listercorev1.NamespaceLister
	namespaceListerSynced cache.InformerSynced
}

// NewK8SController constructs a new instance of K8SController
func NewK8SController(client *kubernetes.Clientset,
	secretInformer informercorev1.SecretInformer,
	namespaceInformer informercorev1.NamespaceInformer) *K8SController {
	c := &K8SController{
		secretGetter:          client.CoreV1(),
		secretLister:          secretInformer.Lister(),
		secretListerSynced:    secretInformer.Informer().HasSynced,
		namespaceGetter:       client.CoreV1(),
		namespaceLister:       namespaceInformer.Lister(),
		namespaceListerSynced: namespaceInformer.Informer().HasSynced,
	}

	secretInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				c.onAdd(obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				c.onUpdate(oldObj, newObj)
			},
			DeleteFunc: func(obj interface{}) {
				c.onDelete(obj)
			},
		},
	)

	return c
}

func (c *K8SController) Run(stop <-chan struct{}) {
	log.Print("waiting for cache sync")
	if !cache.WaitForCacheSync(stop, c.secretListerSynced) {
		log.Print("timed out waiting for cache sync")
		return
	}
	log.Print("caches are synced")

	// wait until we're told to stop
	log.Print("waiting for stop signal")
	<-stop
	log.Print("received stop signal")
}

func (c *K8SController) onAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		log.Printf("onAdd: error getting key for: %#v: %v", obj, err)
		runtime.HandleError(err)
	}
	log.Printf("onAdd: %v", key)
	c.handleSecretChange(obj)
}

func (c *K8SController) onUpdate(oldObj, newObj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(oldObj)
	if err != nil {
		log.Printf("onUpdate: error getting key for %#v: %v", oldObj, err)
		runtime.HandleError(err)
	}
	log.Printf("onUpdate: %v", key)
	c.handleSecretChange(newObj)
}

func (c *K8SController) onDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
	}
	log.Printf("onDelete: %v", key)
	c.handleSecretChange(obj)
}

func (c *K8SController) handleSecretChange(obj interface{}) {
	secret, ok := obj.(*apicorev1.Secret)
	if !ok {
		// TODO: this is probably a `DeletedFinalStateUnknown`.  Figure out what
		// to do.
		return
	}

	if secret.ObjectMeta.Namespace != secretSyncSourceNamespace {
		log.Printf("Skipping secret in wrong namespace")
		return
	}

	if secret.Type != secretSyncType {
		log.Printf("Skipping secret of wrong type")
		return
	}

	log.Printf("Do something with this secret")
	nsList, err := c.namespaceGetter.Namespaces().List(metav1.ListOptions{})

	if err != nil {
		log.Printf("Error listeing namespaces: %v", err)
		return
	}

	for _, ns := range nsList.Items {
		nsName := ns.ObjectMeta.Name
		if _, ok := namespaceBlacklist[nsName]; ok {
			log.Printf("Skipping namespace on blacklist: %v", nsName)
			continue
		}
		log.Printf("We should copy %s to namespace %s", secret.ObjectMeta.Name, ns.ObjectMeta.Name)
		c.copySecretToNamespace(secret, nsName)
	}
}

func (c *K8SController) copySecretToNamespace(secret *apicorev1.Secret, nsName string) {
	// TODO:
	// 1. Make a deep copy of the secret
	// 2. Remove things like object version that'll prevent us from writing
	// 3. Write in new namespace
	// 4. Do a create or update for the new object
}
