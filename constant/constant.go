package constant

const (
	NodeNotSchedulableTaint = "app.openyurtio/nodenotschedulable"

	AnnotationKeyNodeAutonomy = "node.beta.openyurt.io/autonomy" // nodeutil.AnnotationKeyNodeAutonomy
	LabelKeyNodePool          = "apps.openyurt.io/nodepool"

	PodAvailableAnnotation = "pod.beta.openyurt.io/available"
	PodAvailableNode       = "node"
	PodAvailablePool       = "pool"

	DelegateHeartBeat = "openyurt.io/delegate-heartbeat"
)
