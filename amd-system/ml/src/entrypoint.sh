#!/bin/bash
set -e

MODEL_PATH=${MODEL_PATH:-/app/models/amd_model.pkl}

# Generate model if it doesn't exist
if [ ! -f "$MODEL_PATH" ]; then
    echo "Model not found at $MODEL_PATH, generating..."
    python src/generate_model.py --output "$MODEL_PATH" --samples 200 --cleanup
fi

echo "Starting AMD ML Service..."
exec python -m uvicorn src.main:app --host 0.0.0.0 --port "${ML_PORT:-5000}"
