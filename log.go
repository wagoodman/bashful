package main

import (
	"log"
	"os"
)

type LogItem struct {
	Name    string
	Message string
}

func logFlusher() {
	//create your file with desired read/write permissions
	f, err := os.OpenFile(Options.LogPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Fatal(err)
	}

	//defer to close when you're done with it, not because you think it's idiomatic!
	defer f.Close()

	//set output of logs to f
	log.SetOutput(f)

	//test case
	log.Println(bold("Started!"))

	for {
		select {
		case logObj := <-LogChan:
			log.Println(bold("Output from :"+logObj.Name) + "\n" + logObj.Message)
		}
	}
}
