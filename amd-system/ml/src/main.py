"""
AMD ML Service - Answering Machine Detection
FastAPI service for audio classification using MFCC + scikit-learn
"""
import os
import time
import pickle
import base64
import logging
from io import BytesIO
from typing import Dict, Any, Optional

import numpy as np
import librosa
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("amd-ml")

app = FastAPI(title="AMD ML Service", version="1.0.0")

# Configuration
MODEL_PATH = os.environ.get("MODEL_PATH", "/app/models/amd_model.pkl")
SAMPLE_RATE = int(os.environ.get("SAMPLE_RATE", "8000"))
N_MFCC = int(os.environ.get("N_MFCC", "13"))

# In-memory model cache
_model = None

def get_model():
    """Load or return cached model"""
    global _model
    if _model is not None:
        return _model

    if os.path.exists(MODEL_PATH):
        with open(MODEL_PATH, "rb") as f:
            _model = pickle.load(f)
        logger.info(f"Loaded model from {MODEL_PATH}")
    else:
        logger.warning(f"Model not found at {MODEL_PATH}, using dummy classifier")
        _model = None
    return _model


def extract_features(audio_data: bytes, sample_rate: int = 8000) -> np.ndarray:
    """Extract MFCC features from raw audio bytes"""
    # Convert bytes to numpy array (assuming 16-bit PCM)
    audio_array = np.frombuffer(audio_data, dtype=np.int16)
    audio_array = audio_array.astype(np.float32) / 32768.0

    # Normalize
    if len(audio_array) == 0:
        return np.zeros(N_MFCC)

    # Extract MFCC features
    mfcc = librosa.feature.mfcc(
        y=audio_array,
        sr=sample_rate,
        n_mfcc=N_MFCC,
        n_fft=512,
        hop_length=256,
    )

    # Take mean across time axis
    mfcc_mean = np.mean(mfcc, axis=1)

    # Add some additional statistical features
    mfcc_std = np.std(mfcc, axis=1)
    mfcc_max = np.max(mfcc, axis=1)
    mfcc_min = np.min(mfcc, axis=1)

    features = np.concatenate([mfcc_mean, mfcc_std, mfcc_max, mfcc_min])
    return features


def dummy_classify(features: np.ndarray) -> tuple:
    """Fallback classifier when no model is available"""
    # Simple heuristic: high variance in high frequencies = beep
    energy = np.sum(features[:N_MFCC] ** 2)

    if energy > 1.5:
        return "beep", 0.75
    elif energy > 0.8:
        return "human", 0.65
    elif energy > 0.4:
        return "answering_machine", 0.60
    else:
        return "silence", 0.50


class ClassificationRequest(BaseModel):
    session_id: str
    audio_data: str  # base64 encoded
    sample_rate: int = 8000
    channels: int = 1
    format: str = "pcm_16bit"


class ClassificationResponse(BaseModel):
    result: str
    confidence: float
    features: Optional[Dict[str, Any]] = None
    latency_ms: float


class HealthResponse(BaseModel):
    status: str
    model: str
    version: str
    timestamp: int


@app.get("/health", response_model=HealthResponse)
async def health():
    model = get_model()
    model_name = "sklearn_trained" if model else "dummy_heuristic"
    return HealthResponse(
        status="healthy",
        model=model_name,
        version="1.0.0",
        timestamp=int(time.time()),
    )


@app.post("/classify", response_model=ClassificationResponse)
async def classify(req: ClassificationRequest):
    start_time = time.time()

    try:
        audio_bytes = base64.b64decode(req.audio_data)
    except Exception as e:
        raise HTTPException(status_code=400, detail=f"Invalid audio data: {str(e)}")

    if len(audio_bytes) == 0:
        raise HTTPException(status_code=400, detail="Empty audio data")

    # Extract features
    features = extract_features(audio_bytes, req.sample_rate)

    # Classify
    model = get_model()
    if model:
        try:
            features_2d = features.reshape(1, -1)
            prediction = model.predict(features_2d)[0]
            proba = model.predict_proba(features_2d)[0]
            confidence = float(np.max(proba))
            result = str(prediction)
        except Exception as e:
            logger.error(f"Model prediction failed: {e}, falling back to dummy")
            result, confidence = dummy_classify(features)
    else:
        result, confidence = dummy_classify(features)

    latency = (time.time() - start_time) * 1000

    logger.info(f"Session {req.session_id}: {result} (confidence: {confidence:.2f}, latency: {latency:.1f}ms)")

    return ClassificationResponse(
        result=result,
        confidence=round(confidence, 4),
        features={
            "mfcc_dim": len(features),
            "audio_samples": len(audio_bytes) // 2,  # 16-bit = 2 bytes per sample
        },
        latency_ms=round(latency, 2),
    )


@app.post("/classify/realtime", response_model=ClassificationResponse)
async def classify_realtime(req: ClassificationRequest):
    """Optimized endpoint for streaming/realtime classification"""
    return await classify(req)


if __name__ == "__main__":
    import uvicorn
    port = int(os.environ.get("ML_PORT", "5000"))
    uvicorn.run(app, host="0.0.0.0", port=port)
