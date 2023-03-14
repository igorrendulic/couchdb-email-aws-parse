package awshandler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/awstesting/unit"
	"github.com/aws/aws-sdk-go/service/s3"
)

// mocking aws s3 client and session
func dlLoggingSvc(data []byte) (*s3.S3, *[]string, *[]string) {
	var m sync.Mutex
	names := []string{}
	ranges := []string{}

	svc := s3.New(unit.Session)
	svc.Handlers.Send.Clear()
	svc.Handlers.Send.PushBack(func(r *request.Request) {
		m.Lock()
		defer m.Unlock()

		names = append(names, r.Operation.Name)
		ranges = append(ranges, *r.Params.(*s3.GetObjectInput).Range)

		rerng := regexp.MustCompile(`bytes=(\d+)-(\d+)`)
		rng := rerng.FindStringSubmatch(r.HTTPRequest.Header.Get("Range"))
		start, _ := strconv.ParseInt(rng[1], 10, 64)
		fin, _ := strconv.ParseInt(rng[2], 10, 64)
		fin++

		if fin > int64(len(data)) {
			fin = int64(len(data))
		}

		bodyBytes := data[start:fin]
		r.HTTPResponse = &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(bytes.NewReader(bodyBytes)),
			Header:     http.Header{},
		}
		r.HTTPResponse.Header.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d",
			start, fin-1, len(data)))
		r.HTTPResponse.Header.Set("Content-Length", fmt.Sprintf("%d", len(bodyBytes)))
	})

	return svc, &names, &ranges
}

func LoadPayload(filename string) ([]byte, error) {
	jsonFile, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close()
	byteValue, _ := ioutil.ReadAll(jsonFile)
	return byteValue, nil
}

func TestAwsHandlerReceivedMsgParse(t *testing.T) {
	payload, err := LoadPayload("test_data/received.json")
	if err != nil {
		t.Fatal(err)
	}
	// should map to MessageJSON
	var msg MessageJSON
	err = json.Unmarshal(payload, &msg)
	if err != nil {
		t.Fatalf("received.json expected to map to MessageJSONerr: %v\n", err)
	}
}

func TestAwsHandlerReceivedMsgHandle(t *testing.T) {
	payload, err := LoadPayload("test_data/received.json")
	if err != nil {
		t.Fatal(err)
	}

	svc, _, _ := dlLoggingSvc([]byte{})
	smtpHandler := NewAwsSmtpHandler(svc)

	mailReceived, err := smtpHandler.HandleSmtp(payload)
	if err != nil {
		t.Fatalf("received.json expected to be handled without error: %v\n", err)
	}
	if mailReceived.NotificationType != "Received" {
		t.Fatalf("notification type exepcted to be Receive")
	}
}

func TestAwsHandlerBounce(t *testing.T) {
	payload, err := LoadPayload("test_data/bounce.json")
	if err != nil {
		t.Fatal(err)
	}

	svc, _, _ := dlLoggingSvc([]byte{})
	smtpHandler := NewAwsSmtpHandler(svc)

	mailReceived, err := smtpHandler.HandleSmtp(payload)
	if err != nil {
		t.Fatalf("bounce.json expected to be handled without error: %v\n", err)
	}
	if mailReceived.NotificationType != "Bounce" {
		t.Fatalf("notification type exepcted to be Bounce")
	}
}

func TestAwsHandlerDelivery(t *testing.T) {
	payload, err := LoadPayload("test_data/delivery.json")
	if err != nil {
		t.Fatal(err)
	}

	svc, _, _ := dlLoggingSvc([]byte{})
	smtpHandler := NewAwsSmtpHandler(svc)

	mailReceived, err := smtpHandler.HandleSmtp(payload)
	if err != nil {
		t.Fatalf("delivery.json expected to be handled without error: %v\n", err)
	}
	if mailReceived.NotificationType != "Delivery" {
		t.Fatalf("notification type exepcted to be Delivery")
	}
}

func TestAwsHandlerComplaint(t *testing.T) {
	payload, err := LoadPayload("test_data/complaint.json")
	if err != nil {
		t.Fatal(err)
	}

	svc, _, _ := dlLoggingSvc([]byte{})
	smtpHandler := NewAwsSmtpHandler(svc)

	mailReceived, err := smtpHandler.HandleSmtp(payload)
	if err != nil {
		t.Fatalf("delivery.json expected to be handled without error: %v\n", err)
	}
	if mailReceived.NotificationType != "Complaint" {
		t.Fatalf("notification type exepcted to be Delivery")
	}
}
