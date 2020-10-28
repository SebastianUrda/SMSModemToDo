package main

import (
	"encoding/json"
	"flag"
	"github.com/warthog618/modem/at"
	"github.com/warthog618/modem/gsm"
	"github.com/warthog618/modem/serial"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"
)


func main() {
	dev := flag.String("d", "COM5", "path to modem device")
	baud := flag.Int("b", 115200, "baud rate")
	period := flag.Duration("p", 120*time.Minute, "period to wait")
	timeout := flag.Duration("t", 400*time.Millisecond, "command timeout period")
	//verbose := flag.Bool("v", false, "log modem interactions")
	//hex := flag.Bool("x", false, "hex dump modem responses")
	//vsn := flag.Bool("version", false, "report version and exit")
	flag.Parse()
	//if *vsn {
	//	fmt.Printf("%s %s\n", os.Args[0], version)
	//	os.Exit(0)
	//}
	m, err := serial.New(serial.WithPort(*dev), serial.WithBaud(*baud))
	if err != nil {
		log.Println(err)
		return
	}
	defer m.Close()
	var mio io.ReadWriter = m
	//if *hex {
	//	mio = trace.New(m, trace.WithReadFormat("r: %v"))
	//} else if *verbose {
	//	mio = trace.New(m)
	//}
	g := gsm.New(at.New(mio, at.WithTimeout(*timeout)))
	err = g.Init()
	if err != nil {
		log.Println(err)
		return
	}

	//go pollSignalQuality(g)
	go checkForMessages(g)
	err = g.StartMessageRx(
		func(msg gsm.Message) {
			saveToDo(msg, g)
		},
		func(err error) {
			log.Printf("err: %v\n", err)
		})
	if err != nil {
		log.Println(err)
		return
	}
	defer g.StopMessageRx()

	for {
		select {
		case <-time.After(*period):
			log.Println("exiting...")
			return
		case <-g.Closed():
			log.Fatal("modem closed, exiting...")
		}
	}
}

func saveToDo(msg gsm.Message, g *gsm.GSM) {
	splittedMessage := strings.Split(msg.Message, "\n")
	todoText := splittedMessage[0]
	const layout = "2006-01-02T15:04"
	at, _ := time.Parse(layout, splittedMessage[1])
	todo := Todo{Text: todoText,
		Timestamp: at,PhoneNo:msg.Number }
	jsonFile, err := os.Open("todos.json")
	if err != nil {
		log.Println(err)
	}
	byteValue, _ := ioutil.ReadAll(jsonFile)
	jsonFile.Close()
	var todos []Todo
	json.Unmarshal(byteValue, &todos)
	todos = append(todos, todo)
	file, _ := json.Marshal(todos)
	_ = ioutil.WriteFile("todos.json", file, 0644)
	g.SendShortMessage(msg.Number, "Reminder set! \nYou have to do "+todoText+" on "+at.String())

}

type Todo struct {
	Text      string    `json:"text"`
	Timestamp time.Time `json:"timestamp"`
	PhoneNo string `json:"phoneNo"`
}


func checkForMessages(g *gsm.GSM) {
	log.Println("Started")
	for {
		select {
		case <-time.After(3* time.Minute):
			jsonFile, err := os.Open("todos.json")
			if err != nil {
				log.Println(err)
			}
			log.Println("Checking")
			var todos []Todo
			byteValue, _ := ioutil.ReadAll(jsonFile)

			json.Unmarshal(byteValue, &todos)
			for i:=0; i< len(todos); i++ {
				difference := todos[i].Timestamp.Sub(time.Now().UTC().Add(2*time.Hour))
				if difference.Minutes() <= 4  {
					g.SendShortMessage(todos[i].PhoneNo, "You have to "+todos[i].Text+" on "+todos[i].Timestamp.String())
					todos = append(todos[:i], todos[i+1:]...)
					i--
				}
			}

			file, _ := json.Marshal(todos)
			_ = ioutil.WriteFile("todos.json", file, 0644)
			jsonFile.Close()
		}
	}
}

