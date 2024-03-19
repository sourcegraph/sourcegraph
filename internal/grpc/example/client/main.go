package main

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"strconv"
	"sync"
	"time"

	"github.com/sourcegraph/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/sourcegraph/sourcegraph/internal/grpc/defaults"
	pb "github.com/sourcegraph/sourcegraph/internal/grpc/example/weather/v1"
	"github.com/sourcegraph/sourcegraph/internal/grpc/retry"
	"github.com/sourcegraph/sourcegraph/internal/grpc/streamio"
)

// From https://commons.wikimedia.org/w/index.php?title=File:Sun-soleil.svg&oldid=456041378
//
//go:embed sun.svg
var sunDrawingImage []byte

func main() {
	log.Init(log.Resource{
		Name: "weather-client",
	})

	logger := log.Scoped("weather-client")

	// Initialize a gRPC client

	// The defaults.Dial function is a helper function that sets up a gRPC connection with good defaults, including a few monitoring and tracing middlewares.
	// See internal/grpc/defaults/defaults.go for more information.
	conn, err := defaults.Dial("127.0.0.1:50051", logger)
	if err != nil {
		logger.Fatal("Failed to connect", log.Error(err))
	}
	defer conn.Close()

	client := pb.NewWeatherServiceClient(conn)
	client = &automaticRetryClient{base: client} // Wrap the client with our automatic retry logic

	// Demonstrate Unary RPCs (single request, single response): Get weather for a specific location
	//
	// This example demonstrates a basic client server RPC, as well as error handling tactics.
	{

		// You can pass call options to each unique RPC invocation.
		// In this example, we use this to demonstrate the automatic retry logic.
		retryCallback := func(ctx context.Context, attempt uint, err error) {
			logger.Info("The call to GetCurrentWeather for New York failed. Retrying...", log.Uint("attempt", attempt), log.Error(err))
		}

		// Normal case
		weather, err := client.GetCurrentWeather(context.Background(), &pb.GetCurrentWeatherRequest{Location: "New York"}, retry.WithOnRetryCallback(retryCallback))
		if err != nil {
			logger.Fatal("Could not get weather", log.Error(err))
		}

		// We use the generated getter method to safety access the location
		// since there are no required fields in Protobuf messages:
		// The getters return the zero value for the type if the field is not set.
		//
		// See https://protobuf.dev/programming-guides/field_presence/ and https://stackoverflow.com/a/42634681 for more information.
		d, t := weather.GetDescription(), weather.GetTemperature().GetValue()
		logger.Debug("Weather in NYC", log.String("description", d), log.Float64("temperature", t))

		// Error case - get weather for a specific location (that doesn't exist for didactic purposes)
		_, err = client.GetCurrentWeather(context.Background(), &pb.GetCurrentWeatherRequest{Location: "Ravenholm"})

		logger.Debug("This is what a gRPC status error looks like", log.Error(err))
		if status.Code(err) != codes.InvalidArgument { // You can extract the error code from the error object using the status.Code function, and then assert on it.
			logger.Fatal("Expected InvalidArgument error for going to Ravenholm", log.Error(err), log.String("code", status.Code(err).String()))
		}

		weather, err = client.GetCurrentWeather(context.Background(), &pb.GetCurrentWeatherRequest{Location: "Black Mesa"})
		s := status.Convert(err)
		if s.Code() != codes.Internal {
			logger.Fatal("Expected Internal error for going to Black Mesa", log.Error(err), log.String("code", s.Code().String()))
		}

		for _, d := range s.Details() {
			switch info := d.(type) {
			// You can also extract the error details from the error object using the status.Details function, and then assert on it.
			// This allows you to pass arbitrary structs over the wire with as much context as you need to do something in the application.
			case *pb.SensorOfflineError:
				id, message := info.GetSensorId(), info.GetMessage()
				logger.Debug("Sensor is offline", log.String("sensor_id", id), log.String("message", message))
			default:
				// If you don't recognize the error detail, you can log it / ignore it and move on.
				// This is helpful for forwards compatibility
				// (newer server versions may send new error details that older clients don't know about).
				logger.Debug("Unexpected error detail", log.String("info", fmt.Sprintf("%v", info)))
			}
		}
	}

	// Demonstrate that all protobuf string fields must be utf-8 encoded.
	{
		// The protobuf spec says that all strings must be utf-8 encoded (https://protobuf.dev/programming-guides/proto3/#scalar).
		// ("A string must always contain UTF-8 encoded or 7-bit ASCII text ...").
		//
		// When sending protobuf strings that come from user input,
		// you need to ensure that they are utf-8 encoded or otherwise sanitized. Otherwise, the gRPC library will return an error.
		//
		// If you can't guarantee that the string is utf-8 encoded, you can try to switch the type of the underlying field to bytes instead.
		// See https://github.com/sourcegraph/zoekt/pull/641 for an example of this.
		//
		// See https://github.com/sourcegraph/sourcegraph/issues/52181 for more context.
		locationFromUnsanitizedUserInput := "\x80\x80\x80\x80\x80\x80\x80\x80\x80\x80\x80\x80\x80\x80\x80"
		weather, err := client.GetCurrentWeather(context.Background(), &pb.GetCurrentWeatherRequest{Location: locationFromUnsanitizedUserInput})
		if err == nil {
			logger.Fatal("Expected error for invalid utf-8 input, got nil", log.String("location", locationFromUnsanitizedUserInput), log.String("weather", weather.GetDescription()))
		}

		logger.Debug("got expected utf-8 string error when sending in unsanitized user input", log.Error(err))
	}

	// Server Streaming RPC (single request, series of responses): Subscribe to weather alerts for a specific region
	{

		// GRPC supports deadlines, which are a way to set a timeout for a request.
		deadlineCtx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second)) // Set a deadline for the RPC
		defer cancel()

		alertStream, err := client.SubscribeWeatherAlerts(deadlineCtx, &pb.SubscribeWeatherAlertsRequest{Region: "Midwest"})
		if err != nil {
			logger.Fatal("Failed to subscribe to weather alerts", log.Error(err))
		}

		// We use a for loop to receive messages from the server until one of the following happens:
		//	- until the deadline we set above is exceeded
		//	- until the server closes the stream
		//	- until we cancel the RPC ourselves
		for {
			select {
			case <-deadlineCtx.Done():
				goto subscriptionDone // The deadline was exceeded

			default:
				alert, err := alertStream.Recv()
				if err == io.EOF {
					goto subscriptionDone // The server closed the stream
				}

				if deadlineCtx.Err() != nil {
					goto subscriptionDone // We canceled the RPC ourselves
				}

				if err != nil {
					logger.Fatal("Error while receiving alert", log.Error(err))
				}

				logger.Debug("Alert", log.String("alert", alert.GetAlert()))
			}
		}
	}

subscriptionDone:

	// Client Streaming RPC (send multiple request messages, receive a single response message): Upload fake weather data from sensors
	//
	// (Client streaming RPCs are useful for sending data to the server in chunks, such as uploading incrementally produced data)
	{
		dataStream, err := client.UploadWeatherData(context.Background())
		if err != nil {
			logger.Fatal("Error on upload weather data", log.Error(err))
		}

		for range 5 {
			err := dataStream.Send(&pb.UploadWeatherDataRequest{
				SensorId: "sensor-123",
				Temperature: &pb.Temperature{
					Value: 26.5,
					Unit:  pb.Temperature_UNIT_CELSIUS,
				},
				Humidity: 80.0,
			})
			if err != nil {
				logger.Fatal("Error while sending data", log.Error(err))
			}

			time.Sleep(time.Second)
		}

		// CloseAndRecv closes our end of the stream (indicating that we're done sending),
		// and returns the response from the server.
		uploadStatus, err := dataStream.CloseAndRecv()
		if err != nil {
			logger.Fatal("Error while receiving upload status", log.Error(err))
		}

		logger.Debug("Upload status", log.String("message", uploadStatus.GetMessage()))
	}

	// Client Streaming RPC (send multiple request messages, receive a single response message):
	// Upload a byte stream of a weather photo
	//
	// This examples demonstrates how to send a byte stream to the server in chunks using our helper package "streamio".
	{
		stream, err := client.UploadWeatherPhoto(context.Background())
		if err != nil {
			logger.Fatal("Error on upload weather photo", log.Error(err))
		}

		// First send the image metadata
		err = stream.Send(&pb.UploadWeatherPhotoRequest{
			Content: &pb.UploadWeatherPhotoRequest_Metadata_{
				Metadata: &pb.UploadWeatherPhotoRequest_Metadata{
					Location: "Philadelphia",
					SensorId: "sensor-123",
					FileName: "sun.svg",
				},
			},
		})
		if err != nil {
			logger.Fatal("Error while sending image metadata", log.Error(err))
		}

		// Send the image data in chunks.
		//
		// We have a helper package named "streamio" that abstracts away the details of sending a byte stream over gRPC behind
		// a standard io.Writer interface.
		//
		// Notably, it chunks a byte stream into _smaller_ gRPC messages that can be incrementally handled by the server.
		//
		// gRPC wasn’t optimized for sending large messages.
		//
		// Quoting from Microsoft’s “Performance Best Practices with gRPC” page (https://learn.microsoft.com/en-us/aspnet/core/grpc/performance?view=aspnetcore-8.0):
		//
		//    gRPC is a message-based RPC framework, which means:
		//
		//       - The entire message is loaded into memory before gRPC can send it.
		//       - When the message is received, the entire message is deserialized into memory.
		//
		//  This means that you must take care to ensure that your individual gRPC messages are not too large, otherwise you could take down an unsuspecting client or server.
		//  The chunking strategy here (employed by streamio's internals) is one way to do that. See https://handbook.sourcegraph.com/departments/engineering/dev/tools/grpc/#grpc-message-size-limit for more details.
		writer := streamio.NewWriter(func(p []byte) error {
			return stream.Send(&pb.UploadWeatherPhotoRequest{
				Content: &pb.UploadWeatherPhotoRequest_Payload_{
					Payload: &pb.UploadWeatherPhotoRequest_Payload{
						Data: p,
					},
				},
			})
		})

		// Send the image data
		_, err = writer.Write(sunDrawingImage)
		if err != nil {
			logger.Fatal("Error while sending image data", log.Error(err))
		}
		response, err := stream.CloseAndRecv()
		if err != nil {
			logger.Fatal("Error while receiving upload status", log.Error(err))
		}

		logger.Debug("Image upload status", log.String("message", response.GetMessage()))
	}

	// Bidirectional Streaming RPC (send multiple request messages, receive multiple response messages): Get real-time weather updates
	// as you move around.
	//
	// This is a relatively simple example of a bidirectional streaming RPC where we have a single request and response pattern. You can implement
	// more complex semantics (perhaps with multiple request and response messages / types) on top of this.
	{
		biStream, err := client.RealTimeWeather(context.Background())
		if err != nil {
			logger.Fatal("Error on real-time weather", log.Error(err))
		}

		var wg sync.WaitGroup
		wg.Add(1)
		go func() { // Receive messages from the server in a separate goroutine
			defer wg.Done()

			for {
				weather, err := biStream.Recv()
				if err == io.EOF {
					return
				}
				if err != nil {
					logger.Fatal("Error while receiving weather", log.Error(err))
					return
				}

				logger.Debug("Real-time weather", log.String("weather", weather.GetDescription()))
			}
		}()

		// send location information to the server
		for i := range 5 {
			err := biStream.Send(&pb.RealTimeWeatherRequest{
				Location: "Location " + strconv.Itoa(i),
			})
			if err != nil {
				logger.Fatal("Error while sending location update", log.Error(err))
			}
			time.Sleep(2 * time.Second)
		}

		err = biStream.CloseSend() // Close our end of the stream once we're done moving around
		if err != nil {
			logger.Fatal("Error while closing client end of bidirectional stream", log.Error(err))
		}

		wg.Wait() // Wait until we've received all the weather updates back from the server
	}
}
