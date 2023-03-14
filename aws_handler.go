package awshandler

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/igorrendulic/couchdb-email-aws-parse/sns"
	"github.com/igorrendulic/couchdb-experiment/email/mime/handler"
)

// errors
var S3FileNotFoundError = errors.New("s3 mime content file not found")
var SignatureInvalid = errors.New("could not verify signature")

type AwsSmtpHandler struct {
	svc s3iface.S3API
}

func NewAwsSmtpHandler(svc s3iface.S3API) handler.SmtpHandler {
	return &AwsSmtpHandler{
		svc: svc,
	}
}

// Handle SMTP SNS topic notification received by AWS SES (Simple Email Service)
func (p *AwsSmtpHandler) HandleSmtp(message []byte) (*handler.MailReceived, error) {

	var commonMessage map[string]interface{}
	unmErr := json.Unmarshal(message, &commonMessage)
	if unmErr != nil {
		return nil, unmErr
	}

	// subscription confirmation handling (confirming by visiting SubscribeURL in the confirmation message)
	if commonMessage["Type"] == "SubscriptionConfirmation" {
		var notificationPayload sns.Payload
		err := json.Unmarshal(message, &notificationPayload)
		if err != nil {
			return nil, err
		}

		verifyErr := notificationPayload.VerifyPayload()
		if verifyErr != nil {
			return nil, SignatureInvalid
		}

		ts, tsErr := time.Parse(time.RFC3339, notificationPayload.Timestamp)
		if tsErr != nil {
			return nil, tsErr
		}

		if notificationPayload.Type == "SubscriptionConfirmation" {
			_, err := notificationPayload.Subscribe()
			if err != nil {
				return nil, err
			}
			return &handler.MailReceived{
				NotificationType: notificationPayload.Type,
				Timestamp:        ts.UnixMilli(),
			}, nil
		} else {
			return nil, errors.New("unexpected notification type")
		}
	}

	// handling all other SES message types
	var messageJson MessageJSON
	errMj := json.Unmarshal(message, &messageJson)
	if errMj != nil {
		return nil, errMj
	}

	output := &handler.MailReceived{}
	output.NotificationType = messageJson.NotificationType
	output.Timestamp = time.Now().UnixMilli()

	if messageJson.NotificationType == "Received" {
		mail := messageJson.Mail
		receipt := messageJson.Receipt

		bucket, key, bkErr := p.extractS3PathToContent(receipt)
		if bkErr != nil {
			return nil, bkErr
		}
		mimeBytes, mErr := p.downloadS3File(p.svc, bucket, key)
		if mErr != nil {
			return nil, mErr
		}

		output = p.augmentWithMail(output, mail, mimeBytes)

		s3Url := "s3://" + receipt.Action.BucketName
		if receipt.Action.ObjectKeyPrefix != "" {
			s3Url += "/" + receipt.Action.ObjectKeyPrefix
		}
		s3Url += "/" + receipt.Action.ObjectKey

		output.Receipt = &handler.Receipt{
			Action: &handler.Action{
				Type:      receipt.Action.Type,
				Topic:     receipt.Action.TopicArn,
				ObjectURL: s3Url,
			},
			Recipients:           receipt.Recipients,
			ProcessingTimeMillis: receipt.ProcessingTimeMillis,
			SpamVerdict: &handler.VerdictStatus{
				Status: receipt.SpamVerdict.Status,
			},
			VirusVerdict: &handler.VerdictStatus{
				Status: receipt.VirusVerdict.Status,
			},
			SpfVerdict: &handler.VerdictStatus{
				Status: receipt.SpfVerdict.Status,
			},
			DkimVerdict: &handler.VerdictStatus{
				Status: receipt.DkimVerdict.Status,
			},
		}

		return output, nil
	} else if messageJson.NotificationType == "Bounce" {
		bounce := messageJson.Bounce
		mail := messageJson.Mail

		output = p.augmentWithMail(output, mail, nil)

		if bounce != nil {
			recipients := make([]*handler.BouncedRecipient, len(bounce.BouncedRecipients))
			for i, r := range bounce.BouncedRecipients {
				recipients[i] = &handler.BouncedRecipient{
					EmailAddress:   r.EmailAddress,
					Action:         r.Action,
					Status:         r.Status,
					DiagnosticCode: r.DiagnosticCode,
				}
			}
			output.Bounce = &handler.Bounce{
				BounceType:        bounce.BounceType,
				BounceSubType:     bounce.BounceSubType,
				BouncedRecipients: recipients,
				ReportingMTA:      bounce.ReportingMTA,
			}
		}
		return output, nil

	} else if messageJson.NotificationType == "Complaint" {

		complaint := messageJson.Complaint
		mail := messageJson.Mail

		output = p.augmentWithMail(output, mail, nil)

		if complaint != nil {
			recipients := make([]*handler.ComplainedRecipient, len(complaint.ComplainedRecipients))
			for i, r := range complaint.ComplainedRecipients {
				recipients[i] = &handler.ComplainedRecipient{
					EmailAddress: r.EmailAddress,
				}
			}
			output.Complaint = &handler.Complaint{
				UserAgent:             complaint.UserAgent,
				ComplainedRecipients:  recipients,
				ComplaintFeedbackType: complaint.ComplaintFeedbackType,
			}
		}

		return output, nil

	} else if messageJson.NotificationType == "Delivery" {

		delivery := messageJson.Delivery
		mail := messageJson.Mail

		output = p.augmentWithMail(output, mail, nil)

		if delivery != nil {
			ts := time.Now().UnixMilli()
			if delivery.Timestamp != "" {
				// silent fail on parsing timestamp
				t, err := time.Parse(time.RFC3339, delivery.Timestamp)
				if err == nil {
					ts = t.UnixMilli()

				}
			}
			output.Delivery = &handler.Delivery{
				Timestamp:            ts,
				ProcessingTimeMillis: delivery.ProcessingTimeMillis,
				SmtpResponse:         delivery.SmtpResponse,
			}
		}

		return output, nil
	}
	return nil, errors.New("unknown notification type")

}

// augmenting MailReceived with Mail portion of the AWS SNS response
func (p *AwsSmtpHandler) augmentWithMail(output *handler.MailReceived, mail *Mail, mimeBytes []byte) *handler.MailReceived {
	output.Mail = &handler.Mail{
		RawMime:     mimeBytes,
		Source:      mail.Source,
		Destination: mail.Destination,
		MessageID:   mail.MessageID,
	}

	if mail.CommonHeaders != nil {
		output.Mail.CommonHeaders = &handler.CommonHeaders{
			From:      mail.CommonHeaders.From,
			Subject:   mail.CommonHeaders.Subject,
			To:        mail.CommonHeaders.To,
			MessageID: mail.CommonHeaders.MessageID,
		}
	}
	return output
}

func (p *AwsSmtpHandler) extractS3PathToContent(receipt *Receipt) (string, string, error) {
	bucket := ""
	key := ""
	if receipt != nil {
		action := receipt.Action
		if action != nil {
			bucket = action.BucketName
			key = action.ObjectKey
			if action.ObjectKeyPrefix != "" {
				key = action.ObjectKeyPrefix + "/" + action.ObjectKey
			}
		}
	}
	if bucket == "" || key == "" {
		return "", "", S3FileNotFoundError
	}
	return bucket, key, nil
}

func (p *AwsSmtpHandler) downloadS3File(svc s3iface.S3API, bucket string, key string) ([]byte, error) {
	downloader := s3manager.NewDownloaderWithClient(svc)

	buf := aws.NewWriteAtBuffer([]byte{})
	_, err := downloader.Download(buf,
		&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
