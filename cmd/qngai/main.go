package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"qng_agent/internal/qngai"
)

var configPath string

func main() {
	flag.StringVar(&configPath, "config", "config/example.json", "config path")
	flag.Parse()
	// 初始化工作流引擎
	engine, err := qngai.NewWorkflowEngine(configPath)
	if err != nil {
		log.Fatal("Failed to initialize workflow engine:", err)
	}

	// 准备输入数据
	input := map[string]interface{}{
		"user_address": "0x5283DbcBf97bf18558Ee5890646eC5d80E1Fb255",
		"query":        "Analyze my DeFi portfolio",
		"chain_id":     8131,
	}

	// 执行工作流
	ctx := context.Background()
	output, err := engine.Execute(ctx, input)
	if err != nil {
		log.Fatal("Workflow execution failed:", err)
	}

	// 打印结果
	fmt.Printf("Workflow execution completed:\n%+v\n", output)
}
