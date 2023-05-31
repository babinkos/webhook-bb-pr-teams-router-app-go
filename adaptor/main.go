package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/gofiber/fiber/v2/utils"
	"github.com/romana/rlog"
)

type BitBucketUser struct {
	Name         string `json:"name"`
	EmailAddress string `json:"emailAddress"`
	ID           int    `json:"id"`
	DisplayName  string `json:"displayName"`
	Active       bool   `json:"active"`
	Slug         string `json:"slug"`
	Type         string `json:"type"`
	Links        struct {
		Self []struct {
			Href string `json:"href"`
		} `json:"self"`
	} `json:"links"`
}

type BitBucketReviewers []struct {
	User     BitBucketUser `json:"user"`
	Role     string        `json:"role"`
	Approved bool          `json:"approved"`
	Status   string        `json:"status"`
}

type BitBucketPREvent struct {
	EventKey string `json:"eventKey"`
	Date     string `json:"date"`
	Actor    struct {
		Name         string `json:"name"`
		EmailAddress string `json:"emailAddress"`
		ID           int    `json:"id"`
		DisplayName  string `json:"displayName"`
		Active       bool   `json:"active"`
		Slug         string `json:"slug"`
		Type         string `json:"type"`
		Links        struct {
			Self []struct {
				Href string `json:"href"`
			} `json:"self"`
		} `json:"links"`
	} `json:"actor"`
	PullRequest struct {
		ID          int    `json:"id"`
		Version     int    `json:"version"`
		Title       string `json:"title"`
		State       string `json:"state"`
		Open        bool   `json:"open"`
		Closed      bool   `json:"closed"`
		CreatedDate int64  `json:"createdDate"`
		UpdatedDate int64  `json:"updatedDate"`
		FromRef     struct {
			ID           string `json:"id"`
			DisplayID    string `json:"displayId"`
			LatestCommit string `json:"latestCommit"`
			Type         string `json:"type"`
			Repository   struct {
				Slug          string `json:"slug"`
				ID            int    `json:"id"`
				Name          string `json:"name"`
				HierarchyID   string `json:"hierarchyId"`
				ScmID         string `json:"scmId"`
				State         string `json:"state"`
				StatusMessage string `json:"statusMessage"`
				Forkable      bool   `json:"forkable"`
				Project       struct {
					Key         string `json:"key"`
					ID          int    `json:"id"`
					Name        string `json:"name"`
					Description string `json:"description"`
					Public      bool   `json:"public"`
					Type        string `json:"type"`
					Links       struct {
						Self []struct {
							Href string `json:"href"`
						} `json:"self"`
					} `json:"links"`
				} `json:"project"`
				Public bool `json:"public"`
				Links  struct {
					Clone []struct {
						Href string `json:"href"`
						Name string `json:"name"`
					} `json:"clone"`
					Self []struct {
						Href string `json:"href"`
					} `json:"self"`
				} `json:"links"`
			} `json:"repository"`
		} `json:"fromRef"`
		ToRef struct {
			ID           string `json:"id"`
			DisplayID    string `json:"displayId"`
			LatestCommit string `json:"latestCommit"`
			Type         string `json:"type"`
			Repository   struct {
				Slug          string `json:"slug"`
				ID            int    `json:"id"`
				Name          string `json:"name"`
				HierarchyID   string `json:"hierarchyId"`
				ScmID         string `json:"scmId"`
				State         string `json:"state"`
				StatusMessage string `json:"statusMessage"`
				Forkable      bool   `json:"forkable"`
				Project       struct {
					Key         string `json:"key"`
					ID          int    `json:"id"`
					Name        string `json:"name"`
					Description string `json:"description"`
					Public      bool   `json:"public"`
					Type        string `json:"type"`
					Links       struct {
						Self []struct {
							Href string `json:"href"`
						} `json:"self"`
					} `json:"links"`
				} `json:"project"`
				Public bool `json:"public"`
				Links  struct {
					Clone []struct {
						Href string `json:"href"`
						Name string `json:"name"`
					} `json:"clone"`
					Self []struct {
						Href string `json:"href"`
					} `json:"self"`
				} `json:"links"`
			} `json:"repository"`
		} `json:"toRef"`
		Locked bool `json:"locked"`
		Author struct {
			User struct {
				Name         string `json:"name"`
				EmailAddress string `json:"emailAddress"`
				ID           int    `json:"id"`
				DisplayName  string `json:"displayName"`
				Active       bool   `json:"active"`
				Slug         string `json:"slug"`
				Type         string `json:"type"`
				Links        struct {
					Self []struct {
						Href string `json:"href"`
					} `json:"self"`
				} `json:"links"`
			} `json:"user"`
			Role     string `json:"role"`
			Approved bool   `json:"approved"`
			Status   string `json:"status"`
		} `json:"author"`
		Reviewers    BitBucketReviewers `json:"reviewers"`
		Participants []any              `json:"participants"`
		Links        struct {
			Self []struct {
				Href string `json:"href"`
			} `json:"self"`
		} `json:"links"`
	} `json:"pullRequest"`
}

type ReviewerEntity struct {
	Type      string `default:"mention" json:"type"`
	Text      string `json:"text"`
	Mentioned struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"mentioned"`
}

type ReviewerEntitiesList []ReviewerEntity

type TeamsMsgBody struct {
	Type   string `default:"TextBlock" json:"type"`
	Size   string `default:"Medium" json:"size,omitempty"`
	Weight string `default:"Bolder" json:"weight,omitempty"`
	Text   string `default:"Webhook Connector" json:"text"`
	Wrap   bool   `default:"true" json:"wrap"`
}

type TeamsMsgAttachement struct {
	ContentType string `default:"application/vnd.microsoft.card.adaptive" json:"contentType"`
	Content     struct {
		Type    string         `default:"AdaptiveCard" json:"type"`
		Body    []TeamsMsgBody `json:"body"`
		Schema  string         `default:"http://adaptivecards.io/schemas/adaptive-card.json" json:"$schema"`
		Version string         `default:"1.2" json:"version"`
		Msteams struct {
			Width    string               `default:"Full" json:"width"`
			Entities ReviewerEntitiesList `json:"entities"`
		} `json:"msteams"`
	} `json:"content"`
}

type TeamsMsg struct {
	Type        string                `default:"message" json:"type"`
	Attachments []TeamsMsgAttachement `json:"attachments"`
}

// Produce JSON with <> not escaped as unicode
func (t *TeamsMsg) NonEscapedJSON() ([]byte, error) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(t)
	return buffer.Bytes(), err
}

// Parse BitBucket PR event json payload, maps data to build Teams notification webhook json
func ParsePR(eventJson []byte) []byte {
	var inventory BitBucketPREvent
	if err := json.Unmarshal([]byte(eventJson), &inventory); err != nil {
		rlog.Criticalf("Error Unmarshalling payload JSON : %s", err.Error())
		os.Exit(1)
	}
	var reviewersList string = ""
	var reviewersEntity ReviewerEntity
	var reviewersEntityList ReviewerEntitiesList
	reviewersEntity.Type = "mention"
	for _, val := range inventory.PullRequest.Reviewers {
		text := "<at>" + val.User.Name + " UPN</at>"
		reviewersList += text + ", "
		reviewersEntity.Text = text
		reviewersEntity.Mentioned.ID = val.User.EmailAddress
		reviewersEntity.Mentioned.Name = val.User.DisplayName
		reviewersEntityList = append(reviewersEntityList, reviewersEntity)
	}
	reviewersEntity.Mentioned.ID = inventory.PullRequest.Author.User.EmailAddress
	reviewersEntity.Mentioned.Name = inventory.PullRequest.Author.User.DisplayName
	reviewersEntity.Text = "<at>" + inventory.PullRequest.Author.User.Name + " UPN</at>"
	reviewersEntityList = append(reviewersEntityList, reviewersEntity) // add PR author to mentions format
	reviewersList = strings.TrimRight(reviewersList, ", ")
	rlog.Tracef(0, "%+v\n", reviewersEntityList)
	bodyText := fmt.Sprintf("Hi Team, %s %s a PR, please review: [%s](%s) \n\n", reviewersEntity.Text, strings.TrimLeft(inventory.EventKey, "pr:"), inventory.PullRequest.Title, inventory.PullRequest.Links.Self[0].Href)
	bodyText += fmt.Sprintf("CC: %s", reviewersList)
	rlog.Tracef(0, "%s \n", bodyText)

	var msg TeamsMsg
	msg.Type = "message"

	var msgAttachement TeamsMsgAttachement
	msgAttachement.ContentType = "application/vnd.microsoft.card.adaptive"
	msgAttachement.Content.Type = "AdaptiveCard"

	var msgBodyList []TeamsMsgBody
	var msgBody TeamsMsgBody
	msgBody.Type = "TextBlock"
	// msgBody.Size = "Medium"
	// msgBody.Weight = "Bolder"
	msgBody.Text = bodyText
	msgBodyList = append(msgBodyList, msgBody)

	msgAttachement.Content.Body = msgBodyList
	msgAttachement.Content.Schema = "http://adaptivecards.io/schemas/adaptive-card.json"
	msgAttachement.Content.Version = "1.2"
	msgAttachement.Content.Msteams.Width = "Full"
	msgAttachement.Content.Msteams.Entities = reviewersEntityList

	msg.Attachments = append(msg.Attachments, msgAttachement)

	b, err := msg.NonEscapedJSON()
	if err != nil {
		rlog.Errorf("NonEscapedJSON error: %s", err.Error())
	}
	return b
}

func isTraceLevel(tLevel int64) bool {
	return tLevel >= 0
}

const (
	levelNone = iota
	levelCrit
	levelErr
	levelWarn
	levelInfo
	levelDebug
	levelTrace
)

func main() {
	// override fiber encoder/decoder with one provided by goccy/go-json
	app := fiber.New(fiber.Config{
		JSONEncoder:           json.Marshal,
		JSONDecoder:           json.Unmarshal,
		DisableStartupMessage: true,
	})

	os.Setenv("RLOG_LOG_STREAM", "stdout")
	rlog.UpdateEnv()
	var logLevel string = os.Getenv("RLOG_LOG_LEVEL")
	var traceLevelEnv string = os.Getenv("RLOG_TRACE_LEVEL")
	var traceLevel int64
	var teamsHost string = os.Getenv("TEAMS_HOSTNAME") // somecorp.webhook.office.com
	if teamsHost == "" {
		rlog.Critical("Mandatory environment variable TEAMS_HOSTNAME (FQDN from webhook) is not set. You can set it as localhost for development, exiting")
		os.Exit(1)
	} else {
		rlog.Info("TEAMS_HOSTNAME: ", teamsHost)
	}
	if logLevel == "" {
		logLevel = "INFO"
	}
	// If this variable is undefined, or set to -1 then no Trace messages are printed :
	if traceLevelEnv == "" {
		traceLevel = -1
	} else {
		x, err := strconv.ParseInt(traceLevelEnv, 10, 64)
		if err != nil {
			rlog.Criticalf("RLOG_TRACE_LEVEL value provided is not int64 type. Error : %s", err.Error())
			os.Exit(1)
		} else {
			traceLevel = x
		}
	}
	rlog.Infof("RLOG_LOG_LEVEL: %s; RLOG_TRACE_LEVEL: %d", logLevel, traceLevel)

	app.Use(requestid.New(requestid.Config{
		Next:       nil,
		Header:     fiber.HeaderXRequestID,
		Generator:  utils.UUIDv4,
		ContextKey: "requestid",
	}))

	app.Use(logger.New(logger.Config{
		TimeFormat: time.RFC3339,
		Format:     "${time} ACCESS   : [${ip}]:${port} ${locals:requestid} ${status} - ${latency} ${bytesReceived} ${method} ${path}\n",
	}))

	app.Use(compress.New(compress.Config{
		Next: func(c *fiber.Ctx) bool {
			return c.Path() == "/healthz"
		},
		Level: compress.LevelBestSpeed, // 1
	}))

	type SomeStruct struct {
		RequestID string
	}

	// GET /healthz
	app.Get("/healthz", func(c *fiber.Ctx) error {
		return c.SendStatus(204)
	})

	// POST /webhookb2/uid1@uid2/IncomingWebhook/uid3/uid4
	app.Post("/webhookb2/:id1/IncomingWebhook/:id2/:id3", func(c *fiber.Ctx) error {
		c.Accepts("application/json") // "application/json"
		c.AcceptsEncodings("compress", "br")
		data := SomeStruct{
			RequestID: c.GetRespHeader("X-Request-Id"),
		}
		pathid1 := c.Params("id1")
		pathid2 := c.Params("id2")
		pathid3 := c.Params("id3")
		rlog.Debugf("hook ids: %s, %s, %s ; body: %s \n", pathid1, pathid2, pathid3, c.Body())
		notificationBody := ParsePR(c.Body())
		rlog.Debugf("notificationBody : %s", notificationBody)
		// send request to teams , curl -v -X POST -H 'Content-Type: application/json' 'https://somecorp.webhook.office.com/webhookb2/
		a := fiber.AcquireAgent()
		a.ContentType("application/json")
		a.Host(teamsHost)
		req := a.Request()
		req.Header.SetMethod(fiber.MethodPost)
		if errParse := a.Parse(); errParse != nil {
			rlog.Critical("Error during Teams host parsing:" + errParse.Error())
			os.Exit(1)
		}
		if isTraceLevel(traceLevel) {
			a.Debug()
		}

		req.SetRequestURI(fmt.Sprintf("http://%s/webhookb2/%s/IncomingWebhook/%s/%s", teamsHost, pathid1, pathid2, pathid3))
		a.Body(notificationBody)
		a.Add("X-Request-Id", data.RequestID)
		code, body, errs := a.String() // sending request to Teams host
		// moved after RequestURI evaluated and sent because pathid1 was changing after changing Path :
		if (logLevel != "DEBUG") && !(isTraceLevel(traceLevel)) {
			// https://docs.gofiber.io/api/ctx#path :
			// override Path with sha256 encoded webhook credentials
			id1 := fmt.Sprintf("%x", sha256.Sum256([]byte(pathid1)))
			id2 := fmt.Sprintf("%x", sha256.Sum256([]byte(pathid2)))
			id3 := fmt.Sprintf("%x", sha256.Sum256([]byte(pathid3)))
			newPath := fmt.Sprintf("/webhookb2/%s/IncomingWebhook/%s/%s", id1[0:7], id2[0:7], id3[0:7])
			c.Path(newPath) // override to not log sensitive webhook parts
		}
		rlog.Infof("Notification sent to Teams, request Id: %s ; result code:%d", data.RequestID, code)
		if code >= 400 {
			rlog.Errorf("Teams API request (%s) failed with HTTP code: %d", data.RequestID, code)
			return c.SendStatus(code)
		}
		rlog.Debugf("Notification response body:%s", body)
		for i, e := range errs {
			rlog.Errorf("Teams API request (%s) reported errors [%d]: %s \n", data.RequestID, i, e.Error())
			c.SendStatus(504)
		}
		// if errs == nil {
		// 	return c.JSON(data)
		// }
		return c.JSON(data)
	})

	go func() {
		err := app.Listen(":8080")
		if err != nil {
			rlog.Criticalf("Listener on port 8080 error: %s", err.Error())
			os.Exit(1)
		}
	}()

	appHealth := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})
	// GET /healthz
	appHealth.Get("/healthz", func(c *fiber.Ctx) error {
		return c.SendStatus(204)
	})
	errHealthz := appHealth.Listen(":9000")
	if errHealthz != nil {
		rlog.Criticalf("Listener on port 9000 error: %s", errHealthz.Error())
		os.Exit(1)
	}

}