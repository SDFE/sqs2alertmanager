// sqs2alertmanager types to map JSON to Structs
package types

/*

	AWS Message structures

*/

// A AlarmData type is encapsulated in a Cloudwatch Alarm message
type AlarmData struct {
	AWSAccountID     string `json:"AWSAccountId,omitempty"`
	AlarmDescription string `json:"AlarmDescription,omitempty"`
	AlarmName        string `json:"AlarmName,omitempty"`
	NewStateReason   string `json:"NewStateReason,omitempty"`
	NewStateValue    string `json:"NewStateValue,omitempty"`
	OldStateValue    string `json:"OldStateValue,omitempty"`
	Region           string `json:"Region,omitempty"`
	StateChangeTime  string `json:"StateChangeTime,omitempty"`
	Trigger          struct {
		ComparisonOperator string `json:"ComparisonOperator,omitempty"`
		Dimensions         []struct {
			Name  string `json:"name,omitempty"`
			Value string `json:"value,omitempty"`
		} `json:"Dimensions,omitempty"`
		EvaluateLowSampleCountPercentile string      `json:"EvaluateLowSampleCountPercentile,omitempty"`
		EvaluationPeriods                float64     `json:"EvaluationPeriods,omitempty"`
		MetricName                       string      `json:"MetricName,omitempty"`
		Namespace                        string      `json:"Namespace,omitempty"`
		Period                           float64     `json:"Period,omitempty"`
		Statistic                        string      `json:"Statistic,omitempty"`
		StatisticType                    string      `json:"StatisticType,omitempty"`
		Threshold                        float64     `json:"Threshold,omitempty"`
		TreatMissingData                 string      `json:"TreatMissingData,omitempty"`
		Unit                             interface{} `json:"Unit,omitempty"`
	} `json:"Trigger,omitempty"`
}

// A CloudWatchAlert contains the actual Alarm message as json in json
type CloudWatchAlert struct {
	Message          string `json:"Message,omitempty"`
	MessageID        string `json:"MessageId,omitempty"`
	Signature        string `json:"Signature,omitempty"`
	SignatureVersion string `json:"SignatureVersion,omitempty"`
	SigningCertURL   string `json:"SigningCertURL,omitempty"`
	Subject          string `json:"Subject,omitempty"`
	Timestamp        string `json:"Timestamp,omitempty"`
	TopicArn         string `json:"TopicArn,omitempty"`
	Type             string `json:"Type,omitempty"`
	UnsubscribeURL   string `json:"UnsubscribeURL,omitempty"`
	Hello            string `json:"hello,omitempty"`
}

/*

	AlertManager message structures

*/

// A Annotations type is providing additional data for a particular alarm
type Annotations struct {
	Asg          string  `json:"ASG,omitempty"`
	AWSAccountID string  `json:"AWSAccountId,omitempty"`
	Description  string  `json:"Description,omitempty"`
	Reason       string  `json:"Reason,omitempty"`
	Region       string  `json:"Region,omitempty"`
	Source       *string `json:"SqsUrl,omitempty"`
}

// A Labels type provides labels that can be used for grouping alarms in alertmanager
type Labels struct {
	Env        string `json:"env,omitempty"`
	Alertname  string `json:"alertname,omitempty"`
	Region     string `json:"region,omitempty"`
	Service    string `json:"service,omitempty"`
	Severity   string `json:"severity,omitempty"`
	RunbookURL string `json:"runbook,omitempty"`
}

// A AlertmanagerAlert is the combination of Labels and Annotations
type AlertmanagerAlert struct {
	Annotations  Annotations `json:"annotations,omitempty"`
	GeneratorURL string      `json:"generatorURL"`
	Labels       Labels      `json:"labels,omitempty"`
}
