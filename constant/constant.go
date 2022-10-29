package constant

const (
	NodeNotSchedulableTaint = "node.openyurt.io/unschedulable"

	AnnotationKeyNodeAutonomy = "node.beta.openyurt.io/autonomy" // nodeutil.AnnotationKeyNodeAutonomy
	LabelKeyNodePool          = "apps.openyurt.io/nodepool"

	PodAvailableAnnotation = "pod.beta.openyurt.io/available"
	PodAvailableNode       = "node"
	PodAvailablePool       = "pool"

	DelegateHeartBeat = "openyurt.io/delegate-heartbeat"

	LeaseDelegationThreshold = 4

	PoolAliveNodeRatio = 0.3
)
