# AI-Powered Answering Machine Detection

Machine learning pipeline for distinguishing human answers from answering machines to optimize call routing.

## Overview

The AMD System uses a hybrid Go/Python architecture:
- **Go Service**: API gateway, call session management, routing decisions
- **Python ML Service**: Audio classification using MFCC features + scikit-learn

## Architecture

```
Telephony Gateway
    |
    v
Go AMD Service (port 8086)
    |-- Call Manager
    |-- Router
    |-- Classifier Client
    |
    v
Python ML Service (port 5000)
    |-- MFCC Feature Extraction (librosa)
    |-- ML Classification (scikit-learn)
    |-- Real-time Inference
```

## Components

### Go Service
- **Call Manager**: Session lifecycle, audio chunk tracking
- **Classifier Client**: HTTP client for ML service communication
- **Router**: Route calls to agent, voicemail, fax, or retry queue
- **API**: REST endpoints for call management and monitoring

### Python ML Service
- **Feature Extraction**: MFCC (Mel-Frequency Cepstral Coefficients) via librosa
- **Classification**: Trained scikit-learn model (RandomForest/LogisticRegression)
- **Inference**: Real-time classification with confidence scores
- **Fallback**: Heuristic classifier when model unavailable

## Quick Start

```bash
# Start both services
docker-compose up -d

# Or manually:
# Terminal 1: Python ML Service
cd ml && pip install -r requirements.txt
python -m uvicorn src.main:app --host 0.0.0.0 --port 5000

# Terminal 2: Go AMD Service
go run ./cmd
```

## Configuration

### Go Service Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | HTTP API port | `8086` |
| `LOG_LEVEL` | Logging level | `info` |
| `ML_ENDPOINT` | Python ML service URL | `http://localhost:5000` |
| `AGENT_QUEUE` | Route for human detection | `agent_queue` |
| `VOICEMAIL_QUEUE` | Route for answering machine | `voicemail_queue` |
| `FAX_QUEUE` | Route for fax detection | `fax_queue` |
| `RETRY_QUEUE` | Route for unknown/silence | `retry_queue` |

### Python ML Service Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `ML_PORT` | FastAPI port | `5000` |
| `MODEL_PATH` | Path to pickled model | `/app/models/amd_model.pkl` |
| `SAMPLE_RATE` | Audio sample rate | `8000` |
| `N_MFCC` | Number of MFCC features | `13` |

## API Endpoints

### Health Check
```
GET /health
```

### Start Call Session
```
POST /start
```
Request:
```json
{
  "phone_number": "+1-555-0123",
  "campaign_id": "campaign-001"
}
```

### Send Audio Chunk
```
POST /audio
```
Request:
```json
{
  "session_id": "call-xxx",
  "audio_data": "base64_encoded_pcm_audio",
  "sample_rate": 8000,
  "is_final": false
}
```

### Get Classification Result
```
GET /result?session_id=call-xxx
```

### Statistics
```
GET /stats
```

### ML Classifier Health
```
GET /classifier/health
```

## Classification Results

- **`human`**: Live person answered - route to agent
- **`answering_machine`**: Voicemail greeting detected
- **`beep`**: Answering machine beep detected - play message after beep
- **`fax`**: Fax machine tone detected
- **`silence`**: No audio detected - may retry
- **`unknown`**: Cannot determine - retry with more audio

## ML Model Training

To train your own model:

```python
import librosa
import numpy as np
from sklearn.ensemble import RandomForestClassifier
import pickle

def extract_features(audio_path):
    y, sr = librosa.load(audio_path, sr=8000)
    mfcc = librosa.feature.mfcc(y=y, sr=sr, n_mfcc=13)
    mfcc_mean = np.mean(mfcc, axis=1)
    mfcc_std = np.std(mfcc, axis=1)
    mfcc_max = np.max(mfcc, axis=1)
    mfcc_min = np.min(mfcc, axis=1)
    return np.concatenate([mfcc_mean, mfcc_std, mfcc_max, mfcc_min])

# Prepare dataset
X = []
y = []
for path, label in training_data:
    X.append(extract_features(path))
    y.append(label)

X = np.array(X)
y = np.array(y)

# Train model
model = RandomForestClassifier(n_estimators=100, random_state=42)
model.fit(X, y)

# Save model
with open("models/amd_model.pkl", "wb") as f:
    pickle.dump(model, f)
```

## Audio Format

The system expects:
- **Format**: 16-bit PCM
- **Sample Rate**: 8000 Hz (telephony standard)
- **Channels**: Mono (1 channel)
- **Chunks**: 100-500ms recommended for realtime

## Development

### Running Tests

```bash
# Go tests
go test ./... -v

# Python tests (if added)
cd ml && pytest
```

### Building

```bash
# Go binary
go build -o amd-system ./cmd

# Docker
docker-compose build
```

## License

MIT
