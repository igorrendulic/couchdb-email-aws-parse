package awshandler

// var AwsCredCtxKey = &AwsContextKey{"awsCred"}

// type AwsContextKey struct {
// 	Name string
// }

// type AwsCredentials struct {
// 	AwsSecretKey string
// 	AwsAccessKey string
// }

// Amazon SNS Received message parsing for email
type Mail struct {
	Timestamp        string             `json:"timestamp"`
	Source           string             `json:"source"`
	MessageID        string             `json:"messageId"`
	Destination      []string           `json:"destination"`
	HeadersTruncated bool               `json:"headersTruncated"`
	Headers          []*HeaderAttribute `json:"headers"`
	CommonHeaders    *CommonHeader      `json:"commonHeaders"`
}

type ComplainedRecipient struct {
	EmailAddress string `json:"emailAddress" validate:"email,required"`
}

type BouncedRecipient struct {
	EmailAddress   string `json:"emailAddress" validate:"email,required"`
	Status         string `json:"status" vlaidate:"required"`
	Action         string `json:"action" validate:"required"` // failed
	DiagnosticCode string `json:"diagnosticCode"`
}

// optional field, only present if the message was a bounce
type Bounce struct {
	BounceType        string              `json:"bounceType"`              // e.g. Permament
	BounceSubType     string              `json:"bounceSubType,omitempty"` // e.g. General
	ReportingMTA      string              `json:"reportingMTA,omitempty"`  // e.g. "dns; email.example.com", The value of the Reporting-MTA field from the DSN. This is the value of the MTA that attempted to perform the delivery, relay, or gateway operation described in the DSN.
	BouncedRecipients []*BouncedRecipient `json:"bouncedRecipients"`       //  e.g. {"emailAddress":"jane@example.com","status":"5.1.1","action":"failed","diagnosticCode":"smtp; 550 5.1.1 <jane@example.com>... User"}
	RemoteMtaIp       string              `json:"remoteMtaIp,omitempty"`   // e.g. 127.0.0.1" The IP address of the MTA to which Amazon SES attempted to deliver the email.
}

// optional field, only present if the message was a complaint
type Complaint struct {
	UserAgent             string                 `json:"userAgent,omitempty"`             // e.g. AnyCompany Feedback Loop (V0.01)
	ComplainedRecipients  []*ComplainedRecipient `json:"complainedRecipients"`            // e.g. [{"emailAddress":"
	ComplaintFeedbackType string                 `json:"complaintFeedbackType,omitempty"` // e.g. abuse
}

// optional field, only present if the message was delivered (not necessary to use really)
type Delivery struct {
	Timestamp            string `json:"timestamp"`            // miliseconds since epoch
	ProcessingTimeMillis int    `json:"processingTimeMillis"` // miliseconds
	SmtpResponse         string `json:"smtpResponse"`         // e.g. 250 ok:  Message 111 accepted
	ReportingMTA         string `json:"reportingMTA"`         // e.g. a8-70.smtp-out.mail.io
	RemoteMtaIp          string `json:"remoteMtaIp"`          // e.g. 127.0.2.0
}

type MessageJSON struct {
	NotificationType string     `json:"notificationType"`
	Mail             *Mail      `json:"mail,omitempty"`
	Receipt          *Receipt   `json:"receipt,omitempty"`
	Bounce           *Bounce    `json:"bounce,omitempty"`
	Complaint        *Complaint `json:"complaint,omitempty"`
	Delivery         *Delivery  `json:"delivery,omitempty"`
}

type HeaderAttribute struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
type CommonHeader struct {
	ReturnPath string   `json:"returnPath"`
	From       []string `json:"from"`
	Date       string   `json:"date"`
	To         []string `json:"to"`
	MessageID  string   `json:"messageId"`
	Subject    string   `json:"subject"`
}

type Receipt struct {
	Timestamp            string         `json:"timestamp"`
	ProcessingTimeMillis int            `json:"processingTimeMillis"`
	Recipients           []string       `json:"recipients"`
	SpamVerdict          *VerdictStatus `json:"spamVerdict"`
	VirusVerdict         *VerdictStatus `json:"virusVerdict"`
	SpfVerdict           *VerdictStatus `json:"spfVerdict"`
	DkimVerdict          *VerdictStatus `json:"dkimVerdict"`
	DmarcVerdict         *VerdictStatus `json:"dmarcVerdict"`
	Action               *Action        `json:"action"`
}

type Action struct {
	Type            string `json:"type"`
	TopicArn        string `json:"topicArn"`
	BucketName      string `json:"bucketName"`
	ObjectKeyPrefix string `json:"objectKeyPrefix"`
	ObjectKey       string `json:"objectKey"`
}

type VerdictStatus struct {
	Status string `json:"status"`
}

// UserBounceReason - bounce type
type UserBounceReason struct {
	Email        string
	BounceReason string
}

type PlainTextEnvelope struct {
	AwsRef      string                 `json:"awsRef"`
	Subject     string                 `json:"subject"`
	Email       string                 `json:"email"`
	Attachments []*PlainTextAttachment `json:"attachments"`
}

type PlainTextAttachment struct {
	AwsKey      string `json:"awsKey"`
	Size        uint32 `json:"size"`
	Name        string `json:"name"`
	ContentType string `json:"contentType"`
}
