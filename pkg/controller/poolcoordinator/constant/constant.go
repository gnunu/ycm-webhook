package constant

const (
	// nodeutil.AnnotationKeyNodeAutonomy
	AnnotationKeyNodeAutonomy = "node.beta.openyurt.io/autonomy"
	LabelKeyNodePool          = "apps.openyurt.io/nodepool"

	// pod can have two provisioning modes: node bonding, or nodepool bonding
	PodAvailableAnnotation = "pod.beta.openyurt.io/available"
	PodAvailableNode       = "node"
	PodAvailablePool       = "pool"

	DelegateHeartBeat = "openyurt.io/delegate-heartbeat"

	// when node cannot reach api-server directly but can be delegated lease, we should taint the node as unschedulable
	NodeNotSchedulableTaint = "node.openyurt.io/unschedulable"
	// number of lease intervals passed before we taint/detaint node as unschedulable
	LeaseDelegationThreshold = 4

	// when ready nodes in a pool is below this value, we don't allow pod transition any more
	PoolAliveNodeRatio = 0.3
)
