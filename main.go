// sqs2alertmanager app pulls messages from SQS and posts them to Prometheus Alertmanager as a "Critical" alert

/*


	- when should we consider a alert regex valid, when all fields matched, when app and alarmname matched ?


*/

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof" // to provide perf / flamegraphs
	"os"
	"regexp"
	"time"

	"github.com/SDFE/sqs2alertmanager/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/jpillora/backoff"
	"github.com/pingles/go-metrics-riemann"
	"github.com/rcrowley/go-metrics"
	"github.com/rcrowley/go-metrics/exp"
)

var (
	r   = metrics.NewRegistry() // metric registry
	rhc = metrics.NewRegistry() // healthcheck registry
)

// sendMessage sends a formatted JSON document to the Alertmanager
func sendMessage(alertmanagerURL string, b []byte, ch chan *sqs.Message, metricPrefix string, msg *sqs.Message) {

	cERR := metrics.GetOrRegisterCounter(metricPrefix+".alertmanager_err", r)
	cOK := metrics.GetOrRegisterCounter(metricPrefix+".alertmanager_ok", r)

	response, err := http.Post(alertmanagerURL+"/api/v1/alerts", "application/json", bytes.NewBuffer(b))
	if err != nil {
		// ch <- fmt.Sprintf("error: http.Post to %s failed, %v", alertmanagerURL, err)
		cERR.Inc(1)
		close(ch)
		return
	}
	defer response.Body.Close()

	if response.StatusCode < 400 {
		ch <- msg
		cOK.Inc(1)

		// creates a metric for alerts sent to alertmanager per service
		go func() {
			var alertJSONS []*types.AlertmanagerAlert
			err := json.Unmarshal(b, &alertJSONS)
			if err != nil {
				log.Println("error: ", err)
			}
			c := metrics.GetOrRegisterCounter(metricPrefix+".alerts."+alertJSONS[0].Labels.Service, r)
			c.Inc(1)
		}()

	}
	close(ch)
}

// delSqsMessages deletes messages coming in from a *sqs.Message channel
// messages on that channel are already processed and sent off to prometheus-alertmanager, so it is
// save to delete them
func delSqsMessages(ch chan *sqs.Message, queueURL *string, awsEndpoint *string, metricPrefix *string) {

	cERRdel := metrics.GetOrRegisterCounter(*metricPrefix+".sqs_del_error", r)
	cOKdel := metrics.GetOrRegisterCounter(*metricPrefix+".sqs_del_ok", r)

	var sess *session.Session
	var err error

	// TODO: possibly put this somewhere so we only need to have 1 session, rather than multiples, but it works for now
	if *awsEndpoint != "" {
		sess, err = session.NewSession(
			&aws.Config{
				Endpoint:   aws.String(*awsEndpoint),
				DisableSSL: aws.Bool(true),
				Region:     aws.String("us-east-1"),
			})
	} else {
		sess, err = session.NewSession(&aws.Config{})
	}
	if err != nil {
		log.Fatalln("error: unable to create session for deletes: ", *queueURL)
	}

	svc := sqs.New(sess)

	for msg := range ch {
		delMsg := &sqs.DeleteMessageInput{QueueUrl: queueURL, ReceiptHandle: msg.ReceiptHandle}
		_, err := svc.DeleteMessage(delMsg)
		if err != nil {
			cERRdel.Inc(1)
			log.Println("error: ", err)
		}
		cOKdel.Inc(1)
		log.Println("info: deleted", *msg.MessageId)
	}

}

// rcvSqsMessages polls SQS for new messages and retrieves up to 10 messages at a time
func rcvSqsMessages(sqsURL *string, awsEndpoint *string, b *backoff.Backoff, sqsMsgChan chan *sqs.Message, metricPrefix string) {

	cERR := metrics.GetOrRegisterCounter(metricPrefix+".sqs_rcv_error", r)
	cOK := metrics.GetOrRegisterCounter(metricPrefix+".sqs_rcv_ok", r)

	var sess *session.Session
	var err error

	sqsParams := &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(*sqsURL),
		WaitTimeSeconds:     aws.Int64(10), // use long-polling to save money, 10s is max
		VisibilityTimeout:   aws.Int64(120),
		MaxNumberOfMessages: aws.Int64(10),
	}

	// ------------- TODO --------------
	if *awsEndpoint != "" {
		sess, err = session.NewSession(
			&aws.Config{
				Endpoint:   aws.String(*awsEndpoint),
				DisableSSL: aws.Bool(true),
				Region:     aws.String("us-east-1"),
			})
	} else {
		sess, err = session.NewSession(&aws.Config{})
	}
	if err != nil {
		log.Fatalln("error: unable to create session: ", *sqsURL)
	}
	// ------------- TODO END --------------

	svc := sqs.New(sess)

	for {
		rec, err := svc.ReceiveMessage(sqsParams)
		if err != nil {
			cERR.Inc(1)
			log.Println("error: ", err)
			time.Sleep(b.Duration()) // sleep with expo backoff & jitter
			continue
		}

		for _, msg := range rec.Messages {
			cOK.Inc(1)
			log.Println("info: received ", *msg.MessageId)
			sqsMsgChan <- msg
		}

	}

}

// appreadyHandler provides /admin/app-ready
func appreadyHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "OK")
}

// NewHTTPHealthCheck creates a http healthcheck to "url" with name "name"
// http.StatusCode > 499 will return a error
func NewHTTPHealthCheck(url *string, name string, timeout time.Duration) {

	// set 10s timeout for healthchecks to return
	var myClient = &http.Client{Timeout: timeout * time.Second}

	hc := metrics.NewHealthcheck(func(f metrics.Healthcheck) {

		if result, err := myClient.Get(*url); err != nil {
			f.Unhealthy(err)
		} else {
			if result.StatusCode > 499 {
				f.Unhealthy(errors.New(result.Status))
			} else {
				f.Healthy()
			}
		}
	})

	rhc.Register(name, hc)
}

// healthcheckHandler provides /healthcheck endpoint
func healthcheckHandler(w http.ResponseWriter, req *http.Request) {

	// set the correct content type, makes it explicit +
	// http server doesnt have to try and guess the type (and maybe get it wrong)
	w.Header().Add("Content-Type", "application/json")

	// run healthchecks
	rhc.RunHealthchecks()

	// write json response
	enc := json.NewEncoder(w)
	enc.Encode(&rhc)

}

// main function
func main() {

	// awsEndpoint setting needs removing or defaulting to something real
	awsEndpoint := flag.String("endpoint", "", "the aws endpoint URL to use")
	sqsURL := flag.String("sqs", "http://localhost:4100/queue/alerts1", "the sqs queue url")
	alertmanagerURL := flag.String("url", "http://localhost:8080", "the http(s):// url to prometheus alertmanager")
	regex := flag.String("r", ``, "regex to match cloudwatch alerts against")

	// metric / stats config
	listenAddress := flag.String("listen-address", ":8888", "the listen address to serve /debug/metrics, /healthcheck and /admin/app-ready on")
	riemannAddress := flag.String("riemann-host", "localhost:5555", "riemann ip:port")
	metricPrefix := flag.String("metric-prefix", "sqs2alertmanager", "metric prefix to be used, app/servie name would be a good choice")

	// metric output enable/disable
	metricRiemann := flag.Bool("riemann", false, "send metric data to riemann")
	metricOutput := flag.Bool("metrics", false, "output metric data")
	flag.Parse()

	b := &backoff.Backoff{Jitter: true, Min: 5 * time.Second, Max: 5 * time.Minute}

	/*

		create healthchecks

	*/

	NewHTTPHealthCheck(alertmanagerURL, "alertmanager", 2)
	NewHTTPHealthCheck(sqsURL, "aws-sqs", 2)

	/*

		add metrics

	*/

	exp.Exp(r) // expvar + go-metric metrics

	if *metricOutput == true {
		go metrics.Log(r, 60*time.Second, log.New(os.Stderr, "", log.LstdFlags)) // log metrics every 60 seconds
	}

	if *metricRiemann == true {
		go riemann.Report(r, 10*time.Second, *riemannAddress) // send metrics to riemann every 10 seconds
	}

	// serve

	http.HandleFunc("/admin/app-ready", appreadyHandler) // seems to tag along with http.DefaultServeMux
	http.HandleFunc("/healthcheck", healthcheckHandler)
	go http.ListenAndServe(*listenAddress, http.DefaultServeMux) // serve /debug/metrics

	/*


		put sqs messages onto sqsMsgChan channel


	*/

	sqsMsgChan := make(chan *sqs.Message)
	defer close(sqsMsgChan)
	go rcvSqsMessages(sqsURL, awsEndpoint, b, sqsMsgChan, *metricPrefix)

	/*


		consume messages from sqsMsgChan


	*/

	for message := range sqsMsgChan {

		myAlert := types.CloudWatchAlert{}
		err := json.Unmarshal([]byte(*message.Body), &myAlert)
		if err != nil {
			log.Println("error: parsing sqs message: ", err)
			continue
		}

		data := myAlert.Message
		myAlertData := types.AlarmData{}
		err = json.Unmarshal([]byte(data), &myAlertData)
		if err != nil {
			log.Println("error: parsing sqs message.Message: ", err)
			continue
		}

		promAlertAnnotations := types.Annotations{
			Asg:          myAlertData.Trigger.Dimensions[0].Value,
			Description:  myAlertData.AlarmDescription,
			AWSAccountID: myAlertData.AWSAccountID,
			Reason:       myAlertData.NewStateReason,
			Region:       myAlertData.Region,
			Source:       sqsURL,
		}

		// re, err := regexp.Compile(`alert-(?P<env>\w+)-(?P<service>\w+)-(?P<appversion>\d+\-\d+\-\d+\-\d+)-(?P<alarmname>.*)$`)
		re, err := regexp.Compile(*regex)
		if err != nil {
			log.Fatalf("error: unable to compile regex: %v\n", err)
		}
		result := re.FindAllStringSubmatch(myAlertData.AlarmName, -1)
		n1 := re.SubexpNames() // keeps the names of the matches in same order as per regex given

		resultMap := map[string]string{}
		for i, m := range result[0] {
			resultMap[n1[i]] = m
		}

		promAlertLabels := types.Labels{
			Env:        resultMap["env"],
			Alertname:  resultMap["alarmname"],
			Region:     myAlertData.Region,
			Service:    resultMap["service"],
			RunbookURL: resultMap["runbook"],
			Severity:   "Critical", // Cloudwatch has no concept of severity, so everything is critical for now
		}

		promAlert := types.AlertmanagerAlert{
			Annotations:  promAlertAnnotations,
			GeneratorURL: promAlertLabels.RunbookURL,
			Labels:       promAlertLabels,
		}

		var promAlerts []types.AlertmanagerAlert
		promAlerts = append(promAlerts, promAlert)
		jsonMsg, err := json.MarshalIndent(&promAlerts, "", "\t") // convert promAlerts to valid JSON data

		/*

			send the message to alertmanager & delete sqs messages that come back from ch

		*/

		ch := make(chan *sqs.Message)
		delCh := make(chan *sqs.Message)
		defer close(delCh)

		go sendMessage(*alertmanagerURL, jsonMsg, ch, *metricPrefix, message) // send JSON message to alertmanager in a go-routine
		go delSqsMessages(delCh, sqsURL, awsEndpoint, metricPrefix)           // delete messages in a go-routine

		for r := range ch { // consume messages on ch channel
			log.Println("info: processed", *r.MessageId) // log the processed messageId
			delCh <- r                                   // send message to delCh (delete Channel)
		}

	}

}
