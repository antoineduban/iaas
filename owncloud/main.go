/*
 * Nanocloud Community, a comprehensive platform to turn any application
 * into a cloud solution.
 *
 * Copyright (C) 2015 Nanocloud Software
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/rpc/jsonrpc"
	"net/url"
	"os"
	"strings"

	"github.com/natefinch/pie"
	"github.com/streadway/amqp"

	//todo vendor this dependency
	// nan "nanocloud.com/plugins/owncloud/libnan"
)

// Create an object to be exported

var (
	name = "owncloud"
	srv  pie.Server
)

type CreateUserParams struct {
	Username, Password string
}
type Message struct {
	Method    string
	Name      string
	Email     string
	Activated bool
	Sam       string
	Password  string
}

type api struct{}

type PlugRequest struct {
	Body     string
	Header   http.Header
	Form     url.Values
	PostForm url.Values
	Url      string
}

func CreateUser(args PlugRequest, reply *PlugRequest) error {
	var params CreateUserParams
	err := json.Unmarshal([]byte(args.Body), &params)
	if err != nil {
		log.Println(err)
	}
	_, err = Create(params.Username, params.Password)
	if err != nil {
		log.Println(err)
	}
	return err
}

func ChangePassword(args PlugRequest, reply *PlugRequest) {
	var params CreateUserParams
	err := json.Unmarshal([]byte(args.Body), &params)
	if err != nil {
		log.Println(err)
	}
	_, err = Edit(params.Username, "password", params.Password)
}

type del struct {
	Username string
}

func DeleteUser(args PlugRequest, reply *PlugRequest) {

	var User del
	err := json.Unmarshal([]byte(args.Body), &User)
	if err != nil {
		log.Println(err)
	}
	_, err = Delete(User.Username)
	if err != nil {
		log.Println("deletion error: ", err)
	}
}

func (api) Receive(args PlugRequest, reply *PlugRequest) error {
	initConf()
	Configure()

	if strings.Index(args.Url, "/owncloud/add") == 0 {
		CreateUser(args, reply)
	}
	if strings.Index(args.Url, "/owncloud/delete") == 0 {
		DeleteUser(args, reply)
	}
	if strings.Index(args.Url, "/owncloud/changepassword") == 0 {
		ChangePassword(args, reply)
	}

	return nil
}

func (api) Plug(args interface{}, reply *bool) error {
	*reply = true
	go LookForMsg()
	return nil
}

func (api) Check(args interface{}, reply *bool) error {
	*reply = true
	return nil
}

func (api) Unplug(args interface{}, reply *bool) error {
	defer os.Exit(0)
	*reply = true
	return nil
}

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
		panic(fmt.Sprintf("%s: %s", msg, err))
	}
}

func LookForMsg() {
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"users", // name
		false,   // durable
		false,   // delete when usused
		false,   // exclusive
		false,   // no-wait
		nil,     // arguments
	)
	failOnError(err, "Failed to declare a queue")

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	failOnError(err, "Failed to register a consumer")

	forever := make(chan bool)

	go func() {
		var msg Message
		for d := range msgs {
			log.Printf("Received a message: %s", d.Body)
			err := json.Unmarshal(d.Body, &msg)
			if err != nil {
				log.Println(err)
			}
			if msg.Method == "Add" {
				initConf()
				Configure()
				_, err := Create(msg.Name, msg.Password)
				if err != nil {
					log.Println("create error?:")
					log.Println(err)
				}
			}
		}
	}()

	log.Printf(" [*] Waiting for messages. To exit press CTRL+C")
	<-forever
}
func main() {
	srv = pie.NewProvider()

	if err := srv.RegisterName(name, api{}); err != nil {
		log.Fatalf("Failed to register %s: %s", name, err)
	}

	srv.ServeCodec(jsonrpc.NewServerCodec)

}