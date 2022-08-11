package main

import (
	"errors"
	"encoding/json"
	"log"
	"os"
	"fmt"
	"strconv"
	"net/http"
	"io/ioutil"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	
	"github.com/line/line-bot-sdk-go/linebot"
)

var (
	// DefaultHTTPGetAddress Default Address
	DefaultHTTPGetAddress = "https://checkip.amazonaws.com"

	// ErrNoIP No IP found in response
	ErrNoIP = errors.New("No IP in HTTP response")

	// ErrNon200Response non 200 status code in response
	ErrNon200Response = errors.New("Non 200 Response found")
)

type Line struct {
	ChannelSecret string
	ChannelToken  string
	Client        *linebot.Client
}

type response struct {
	Results results `json:"results"`
}

type results struct {
	Shop []shop `json:"shop"`
}

type shop struct {
	Name string `json:"name"`
	Address string `json:"address"`
	Photo photo `json:photo`
	URLS urls `json:urls`
}

type photo struct {
	Mobile mobile `json:mobile`
}

type mobile struct {
	L string `json:"l"`
}

type urls struct {
	PC string `json:"pc"`
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	bot, err := linebot.New(
		os.Getenv("LINE_BOT_CHANNEL_SECRET"),
		os.Getenv("LINE_BOT_CHANNEL_TOKEN"),
	)
	line := Line {
		ChannelSecret: os.Getenv("LINE_BOT_CHANNEL_SECRET"),
		ChannelToken: os.Getenv("LINE_BOT_CHANNEL_TOKEN"),
		Client: bot,
	}

	if err != nil {
		return events.APIGatewayProxyResponse{Body: "接続エラー", StatusCode: 200}, err
	}

	lineEvents, err := ParseRequest(line.ChannelSecret, request)
    if err != nil {
        return events.APIGatewayProxyResponse{Body: "接続エラー", StatusCode: 200}, err
    }

	log.Println("LINEイベント")
	
	for _, event := range lineEvents {
		// イベントがメッセージの受信だった場合
		if event.Type == linebot.EventTypeMessage {
			switch message := event.Message.(type) {
			
			case *linebot.TextMessage:
				log.Println(message)
				replyMessage := message.Text
				_, err = bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage(replyMessage)).Do()
				if err != nil {
					return events.APIGatewayProxyResponse{}, err
				}
				break
			case *linebot.LocationMessage:
				err = sendShopListInfo(line.Client, event)
				if err != nil {
					return events.APIGatewayProxyResponse{}, err
				}
				break		
			default:
			}
		}
	}

	
	// resp, err := http.Get(DefaultHTTPGetAddress)
	// if err != nil {
	// 	return events.APIGatewayProxyResponse{}, err
	// }

	// if resp.StatusCode != 200 {
	// 	return events.APIGatewayProxyResponse{}, ErrNon200Response
	// }

	// ip, err := ioutil.ReadAll(resp.Body)
	// if err != nil {
	// 	return events.APIGatewayProxyResponse{}, err
	// }

	// if len(ip) == 0 {
	// 	return events.APIGatewayProxyResponse{}, ErrNoIP
	// }

	return events.APIGatewayProxyResponse{Body: request.Body, StatusCode: 200}, nil
}

func ParseRequest(channelSecret string, r events.APIGatewayProxyRequest) ([]*linebot.Event, error) {
	req := &struct {
		Events []*linebot.Event `json:"events"`
	}{}
	if err := json.Unmarshal([]byte(r.Body), req); err != nil {
		return nil, err
	}
	return req.Events, nil
}

func sendShopListInfo(lineClient *linebot.Client, e *linebot.Event) error {
	msg := e.Message.(*linebot.LocationMessage)

	lat := strconv.FormatFloat(msg.Latitude, 'f', 2, 64)
	lng := strconv.FormatFloat(msg.Longitude, 'f', 2, 64)

	replyMsg, err := getShopListInfo(lat, lng)
	if err != nil {
		return err
	}

	res := linebot.NewTemplateMessage(
		"ラーメン一覧",
		linebot.NewCarouselTemplate(replyMsg...).WithImageOptions("rectangle", "cover"),
	)

	_, err = lineClient.ReplyMessage(e.ReplyToken, res).Do()

	return err
}

func getShopListInfo(latitude string, longitude string) ([]*linebot.CarouselColumn, error) {
	apikey := fmt.Sprintf("%s", os.Getenv("API_KEY"))

	url := fmt.Sprintf("https://webservice.recruit.co.jp/hotpepper/gourmet/v1/?format=json&genre=G013&range=5&key=%s&lat=%s&lng=%s", apikey, latitude, longitude)
	
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var responseData response
	if err := json.Unmarshal(body, &responseData); err != nil {
		return nil, err
	}

	var ccs []*linebot.CarouselColumn
	for _, shop := range responseData.Results.Shop {
		addr := shop.Address
		// if 60 < utf8.Rune
		cc := linebot.NewCarouselColumn(
			shop.Photo.Mobile.L,
			shop.Name,
			addr,
			linebot.NewURIAction("詳細", shop.URLS.PC),
		).WithImageOptions("#FFFFFF")
		
		ccs = append(ccs, cc)
	}

	return ccs, nil
}

func main() {
	lambda.Start(handler)
}
