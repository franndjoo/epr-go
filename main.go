package main

import (
	"fmt"
	"io"
	"johanmnto/epr/config"
	"johanmnto/epr/helpers"
	"johanmnto/epr/net"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

func main() {
	var serverConfiguration config.EPRConfig = config.LoadAndParseConfiguration()
	var serverWG *sync.WaitGroup = new(sync.WaitGroup)

	fmt.Println("Configuration loaded: using port", serverConfiguration.Server.HttpPort, "for HTTP requests.")
	if serverConfiguration.Server.HttpsPort != nil {
		if serverConfiguration.Server.HttpsKeyPath == nil || serverConfiguration.Server.HttpsCertPath == nil {
			panic("Cannot authorize an HTTPS configuration without having the appropriate certificates loaded.")
		}
		fmt.Println("Found an HTTPS configuration: will use port", *serverConfiguration.Server.HttpsPort, "for HTTPS requests.")
	}

	server := helpers.MakeServer(&serverConfiguration)
	server.MakeHandler(func(w http.ResponseWriter, r *http.Request) {
		if helpers.PointsToKnownTarget(r, &serverConfiguration) {
			headerTargetAsNumber, _ := strconv.Atoi(r.Header.Get(net.TARGET_HEADER_NAME))
			binder, err := net.GenerateBinder(r, serverConfiguration.Bindings[headerTargetAsNumber])

			if err != nil {
				println(err.Error())
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				// Transfers the request to the binded port
				response, err := binder.BindToFromBinder()

				if err != nil {
					println(err.Error())
					w.WriteHeader(http.StatusInternalServerError)
				} else {
					// If the transfert had succeded and a response has been given, the response is copied
					// to be sent back to the client.
					body, err := io.ReadAll(response.Body)

					if err != nil {
						println(err.Error())
						w.WriteHeader(http.StatusInternalServerError)
					} else {
						w.WriteHeader(response.StatusCode)
						for headerName, headerValue := range response.Header {
							w.Header().Add(headerName, strings.Join(headerValue, ", "))
						}
						w.Write(body)
					}
				}
			}

		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})

	// Starts both unsecure and secure server, the secure server is started only if an HTTPS port is provided.
	serverWG.Add(1)
	go func() {
		if err := server.ServeUnsecure(); err != nil {
			panic(fmt.Sprintf("HTTP server failure: %s", err.Error()))
		}
		serverWG.Done()
	}()
	if serverConfiguration.Server.HttpsPort != nil {
		serverWG.Add(1)
		go func() {
			if err := server.ServeSecure(); err != nil {
				panic(fmt.Sprintf("HTTPS server failure: %s", err.Error()))
			}
			serverWG.Done()
		}()
	}

	serverWG.Wait()
}
