package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/go-resty/resty/v2"
)

// Constants
const openaiAPIURL = "https://api.openai.com/v1/chat/completions"

// const model = "gpt-3.5-turbo"
const model = "gpt-4o"

// In-memory storage for appointments
var appointments = map[string]string{}

// Global conversation history
var conversationHistory = []map[string]string{
	{"role": "system", "content": "You are a helpful assistant for Super Clinic's appointment booking system. Parse relative dates like 'today' and 'tomorrow'. Use ISO Date format"},
}

// Get OpenAI API Key from environment variable
var openaiAPIKey = os.Getenv("OPENAI_API_KEY")

func sendMessageToGPT(userMessage string) (string, error) {
	client := resty.New()

	// Append user's message to the conversation history
	conversationHistory = append(conversationHistory, map[string]string{
		"role":    "user",
		"content": userMessage,
	})

	// Payload with functions for OpenAI
	payload := map[string]interface{}{
		"model":    model,
		"messages": conversationHistory,
		"functions": []map[string]interface{}{
			{
				"name":        "checkAvailability",
				"description": "Checks if the doctor is available for a given time slot.",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"doctorName": map[string]interface{}{
							"type":        "string",
							"description": "The name of the doctor.",
						},
						"date": map[string]interface{}{
							"type":        "string",
							"description": "The date for the appointment (YYYY-MM-DD).Use ISO Date format",
						},
						"time": map[string]interface{}{
							"type":        "string",
							"description": "The time for the appointment (HH:MM).",
						},
					},
					"required": []string{"doctorName", "date", "time"},
				},
			},
			{
				"name":        "storeAppointment",
				"description": "Stores an appointment with the specified doctor, date, and time.",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"doctorName": map[string]interface{}{
							"type":        "string",
							"description": "The name of the doctor.",
						},
						"date": map[string]interface{}{
							"type":        "string",
							"description": "The date for the appointment (YYYY-MM-DD). Use ISO Date format",
						},
						"time": map[string]interface{}{
							"type":        "string",
							"description": "The time for the appointment (HH:MM).",
						},
					},
					"required": []string{"doctorName", "date", "time"},
				},
			},
		},
	}

	// Send request to OpenAI
	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", "Bearer "+openaiAPIKey).
		SetBody(payload).
		Post(openaiAPIURL)

	// Print response body and status code for debugging
	//fmt.Println("Response Status Code:", resp.StatusCode())
	//fmt.Println("Response Body:", string(resp.Body()))

	if err != nil {
		return "", err
	}

	// Parse response
	var result map[string]interface{}
	err = json.Unmarshal(resp.Body(), &result)
	if err != nil {
		return "", err
	}

	// Check for function call
	if choices, ok := result["choices"].([]interface{}); ok && len(choices) > 0 {
		choice := choices[0].(map[string]interface{})
		if functionCall, ok := choice["message"].(map[string]interface{})["function_call"]; ok {
			// Extract function name and arguments
			functionName := functionCall.(map[string]interface{})["name"].(string)
			args := functionCall.(map[string]interface{})["arguments"].(string)

			// Parse the arguments JSON
			var functionArgs map[string]string
			if err := json.Unmarshal([]byte(args), &functionArgs); err != nil {
				return "", err
			}

			// Handle function call
			if functionName == "checkAvailability" {
				available := checkAvailability(functionArgs["doctorName"], functionArgs["date"], functionArgs["time"])
				if available {
					return fmt.Sprintf("The doctor is available on %s at %s. Would you like to book this slot?", functionArgs["date"], functionArgs["time"]), nil
				}
				return fmt.Sprintf("Sorry, the doctor is not available on %s at %s.", functionArgs["date"], functionArgs["time"]), nil
			} else if functionName == "storeAppointment" {
				storeAppointment(functionArgs["doctorName"], functionArgs["date"], functionArgs["time"])
				return "Your appointment has been successfully booked.", nil
			}
		} else if content, ok := choice["message"].(map[string]interface{})["content"].(string); ok {
			// Regular message content
			return content, nil
		}
	}

	return "", fmt.Errorf("unexpected response format")
}

func checkAvailability(doctorName, date, time string) bool {
	slotKey := fmt.Sprintf("%s %s %s", doctorName, date, time)
	_, exists := appointments[slotKey]
	return !exists
}

func storeAppointment(doctorName, date, time string) {
	slotKey := fmt.Sprintf("%s %s %s", doctorName, date, time)
	appointments[slotKey] = doctorName
}

func main() {
	fmt.Println("Agent: Hello and welcome to Super Clinic!")

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Caller: ")
		userMessage, _ := reader.ReadString('\n')
		userMessage = strings.TrimSpace(userMessage)

		response, err := sendMessageToGPT(userMessage)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}

		// Append assistant's message to the conversation history
		conversationHistory = append(conversationHistory, map[string]string{
			"role":    "assistant",
			"content": response,
		})

		fmt.Println("Agent: ", response)

		if strings.Contains(strings.ToLower(response), "appointment has been successfully booked") {
			break
		}
	}
}
