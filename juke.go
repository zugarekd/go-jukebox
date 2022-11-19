package main

import (
	"encoding/json"
	"fmt"
	"github.com/hajimehoshi/oto"
	"github.com/talkkonnect/max7219"
	"github.com/tosone/minimp3"
	lcd "github.com/wjessop/lcm1602_lcd"
	"golang.org/x/exp/io/i2c"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

var selection string

var songQueue = make([]string, 0)

var playing string

var title string

var mtx *max7219.Matrix

var lcdDisplay *lcd.LCM1602LCD

func main() {
	mtx = max7219.NewMatrix(1)
	err := mtx.Open(0, 0, 7)
	if err != nil {
		log.Fatal(err)
	}
	defer mtx.Close()

	lcdDevice, err := i2c.Open(&i2c.Devfs{Dev: "/dev/i2c-1"}, 0x27)
	if err != nil {
		log.Fatal(err)
	}
	defer lcdDevice.Close()
	lcdDisplay, err = lcd.NewLCM1602LCD(lcdDevice)
	if err != nil {
		log.Fatal(err)
	}
	lcdDisplay.WriteString("Dan Z's Juke", 1, 0)

	http.HandleFunc("/event", event)
	http.HandleFunc("/current", current)
	go http.ListenAndServe(":8080", nil)

	sigchnl := make(chan os.Signal, 1)
	signal.Notify(sigchnl)
	exitchnl := make(chan int)

	go func() {
		playQueue()
	}()

	go func() {
		for {
			s := <-sigchnl
			handler(s)
		}
	}()

	exitcode := <-exitchnl
	os.Exit(exitcode)
}

func handler(signal os.Signal) {
	if signal == syscall.SIGTERM {
		fmt.Println("Got kill signal. ")
		fmt.Println("Program will terminate now.")
		os.Exit(0)
	} else if signal == syscall.SIGINT {
		fmt.Println("Got CTRL+C signal")
		fmt.Println("Closing.")
		os.Exit(0)
	} else {
		//fmt.Println("Ignoring signal: ", signal)
	}
}

func event(w http.ResponseWriter, req *http.Request) {
	event, _ := req.URL.Query()["event"]

	key, _ := req.URL.Query()["key"]

	intVar, _ := strconv.Atoi(key[0])
	buttonVal := string(intVar)

	if event[0] == "up" && (buttonVal == "1" ||
		buttonVal == "2" ||
		buttonVal == "3" ||
		buttonVal == "4" ||
		buttonVal == "5" ||
		buttonVal == "6" ||
		buttonVal == "7" ||
		buttonVal == "8" ||
		buttonVal == "9" ||
		buttonVal == "0") {
		selection = selection + buttonVal
	} else if event[0] == "up" && (buttonVal == "r" || buttonVal == "R") {
		selection = ""
	}

	if len(selection) == 3 {
		pushSong(selection)
		selection = ""
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(CurrentResponse{Current: "100"})
}

type CurrentResponse struct {
	Current string
}

func current(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	json.NewEncoder(w).Encode(CurrentResponse{Current: getSongDisplay()})
}

func getSongDisplay() string {
	songDisplay := selection
	if len(selection) == 0 || len(selection) >= 3 {
		songDisplay = playing
	}
	if songDisplay == "" {
		mtx.Device.ClearAll(true)
		//lcdDisplay.Clear()
		lcdDisplay.WriteString("Dan Z's Juke", 1, 0)
	} else {
		mtx.Device.SevenSegmentDisplay(songDisplay)
		//lcdDisplay.Clear()

		lcdDisplay.WriteString("Dan Z's Juke", 1, 0)
		if len(title) > 8 {
			lcdDisplay.WriteString(title[0:8], 2, 0)
		} else {
			lcdDisplay.WriteString(title, 2, 0)
		}
	}
	return songDisplay
}

func pushSong(song string) {
	songQueue = append(songQueue, song)
}

func nextSong() string {
	if len(songQueue) == 0 {
		return ""
	}
	rtn := songQueue[0]
	songQueue = songQueue[1:]

	return rtn
}

func playQueue() {
	for {
		time.Sleep(1 * time.Second)
		nxt := nextSong()
		if nxt != "" {
			playSong(nxt)
		}
	}
}

func playSong(slot string) {
	fileName := getSongFile(slot)
	if fileName != "" {
		playing = slot
		// open file
		file, err := os.Open(fileName)
		//file, err := os.Open("example.mp3")
		if err != nil {
			log.Fatalln(err)
		}
		defer file.Close()

		// new a decoder
		dec, err := minimp3.NewDecoder(file)
		if err != nil {
			log.Fatalln(err)
		}
		defer dec.Close()
		<-dec.Started()

		// new a context and a player
		var context *oto.Context
		if context, err = oto.NewContext(dec.SampleRate, dec.Channels, 2, 1024); err != nil {
			log.Fatal(err)
		}
		defer context.Close()
		var player = context.NewPlayer()
		defer player.Close()

		// start playing
		fmt.Println("Starting!")
		io.Copy(player, dec)
	}
	playing = ""
	title = ""
	getSongDisplay()
}

func getSongFile(slot string) string {
	files, err := ioutil.ReadDir("/home/zugarekd/go/src/github.com/zugarekd/go-jukebox/songs/" + slot)
	if err != nil {
		return ""
		//log.Fatal(err)
	}

	for _, file := range files {
		if !file.IsDir() {
			title = file.Name()
			return "/home/zugarekd/go/src/github.com/zugarekd/go-jukebox/songs/" + slot + "/" + file.Name()
		}
		fmt.Println(file.Name(), file.IsDir())
	}
	return ""
}
