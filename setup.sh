#!/bin/bash

# QNG Intelligent Agent Setup Script

echo "🚀 Setting up QNG Intelligent Agent..."

# Create data directory
mkdir -p data

# Install frontend dependencies
echo "📦 Installing frontend dependencies..."
cd frontend
npm install

# Build frontend
echo "🏗️  Building frontend..."
npm run build

# Go back to root
cd ..

# Download Go dependencies
echo "📚 Downloading Go dependencies..."
go mod tidy

# Build the Go backend
echo "🔨 Building Go backend..."
go build -o bin/qng-agent cmd/main.go

echo "✅ Setup complete!"
echo ""
echo "To start the application:"
echo "1. Backend: ./bin/qng-agent"
echo "2. Frontend dev server: cd frontend && npm run dev"
echo ""
echo "Or start both with: ./scripts/start.sh"