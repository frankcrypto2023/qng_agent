#!/bin/bash

# Start script for development

echo "🚀 Starting QNG Intelligent Agent..."

# Start backend in background
echo "📡 Starting backend server..."
./bin/qng-agent &
BACKEND_PID=$!

# Wait a moment for backend to start
sleep 2

# Start frontend dev server
echo "🌐 Starting frontend dev server..."
cd frontend
npm run dev &
FRONTEND_PID=$!

# Function to cleanup on exit
cleanup() {
    echo "🛑 Shutting down servers..."
    kill $BACKEND_PID 2>/dev/null
    kill $FRONTEND_PID 2>/dev/null
    exit 0
}

# Set up signal handlers
trap cleanup SIGINT SIGTERM

echo "✅ Both servers are running!"
echo "📡 Backend: http://localhost:8080"
echo "🌐 Frontend: http://localhost:3000"
echo "Press Ctrl+C to stop both servers"

# Wait for processes
wait