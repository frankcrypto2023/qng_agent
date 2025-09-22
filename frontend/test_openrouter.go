package main

import (
	"context"
	"fmt"
	"log"
	"qng-agent/internal/llm"
	"qng-agent/internal/types"
)

func main() {
	// Test OpenRouter client
	fmt.Println("Testing OpenRouter integration...")
	
	// Create OpenRouter client
	client := llm.NewOpenRouterClient(
		"https://openrouter.ai/api/v1",
		"test-api-key", // Replace with actual API key for testing
		"openai/gpt-4o",
		2048,
		0.7,
		"QNG Agent Test",
		"https://qng-agent.test",
	)
	
	// Test messages
	messages := []types.ChatMessage{
		{
			Role:    "user",
			Content: "Hello, can you help me with blockchain queries?",
		},
	}
	
	// Test completion (this will fail without real API key, but tests the structure)
	ctx := context.Background()
	response, err := client.GetCompletion(ctx, messages)
	if err != nil {
		log.Printf("Expected error (no real API key): %v", err)
	} else {
		fmt.Printf("Response: %s\n", response)
	}
	
	// Test manager with OpenRouter config
	manager := llm.NewManager()
	config := types.LLMProviderConfig{
		Type:      types.ProviderTypeOpenRouter,
		Name:      "OpenRouter",
		URL:       "https://openrouter.ai/api/v1",
		Token:     "test-api-key",
		ModelName: "openai/gpt-4o",
		AppName:   "QNG Agent",
		AppURL:    "https://qng-agent.test",
	}
	
	manager.UpdateClientFromConfig(config)
	fmt.Println("OpenRouter client configured successfully!")
	
	// Test streaming (will also fail without real API key)
	fmt.Println("Testing streaming...")
	err = client.StreamCompletion(ctx, messages, func(chunk string) {
		fmt.Print(chunk)
	}, func() {
		fmt.Println("\n[Stream completed]")
	})
	
	if err != nil {
		log.Printf("Expected error (no real API key): %v", err)
	}
	
	fmt.Println("OpenRouter integration test completed!")
}
