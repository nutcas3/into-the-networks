"""
Evaluate trained AMD model on test audio files.
"""
import os
import argparse
import pickle
import base64
import time

import numpy as np
from train import extract_features, REVERSE_LABELS


def load_model(model_path):
    """Load pickled model"""
    with open(model_path, "rb") as f:
        model = pickle.load(f)
    return model


def classify_file(model, audio_path, sample_rate=8000):
    """Classify a single audio file"""
    features = extract_features(audio_path, sample_rate=sample_rate)
    if features is None:
        return None, 0.0

    features_2d = features.reshape(1, -1)
    prediction = model.predict(features_2d)[0]
    proba = model.predict_proba(features_2d)[0]
    confidence = float(np.max(proba))
    label = REVERSE_LABELS[prediction]

    return label, confidence


def classify_bytes(model, audio_bytes, sample_rate=8000):
    """Classify raw audio bytes (simulating the API)"""
    import io
    import soundfile as sf

    try:
        audio_array, sr = sf.read(io.BytesIO(audio_bytes), dtype="float32")
        if len(audio_array) == 0:
            return None, 0.0

        # Resample if needed
        if sr != sample_rate:
            import librosa
            audio_array = librosa.resample(audio_array, orig_sr=sr, target_sr=sample_rate)

        # Extract features inline (simplified)
        import librosa
        mfcc = librosa.feature.melspectrogram(
            y=audio_array,
            sr=sample_rate,
            n_mels=13,
            n_fft=512,
            hop_length=256,
        )
        mfcc_mean = np.mean(mfcc, axis=1)
        mfcc_std = np.std(mfcc, axis=1)
        mfcc_max = np.max(mfcc, axis=1)
        mfcc_min = np.min(mfcc, axis=1)

        spectral_centroid = librosa.feature.spectral_centroid(y=audio_array, sr=sample_rate)[0]
        spectral_rolloff = librosa.feature.spectral_rolloff(y=audio_array, sr=sample_rate)[0]
        zcr = librosa.feature.zero_crossing_rate(audio_array)[0]

        features = np.concatenate([
            mfcc_mean, mfcc_std, mfcc_max, mfcc_min,
            [np.mean(spectral_centroid), np.std(spectral_centroid)],
            [np.mean(spectral_rolloff), np.std(spectral_rolloff)],
            [np.mean(zcr), np.std(zcr)],
        ])

        features_2d = features.reshape(1, -1)
        prediction = model.predict(features_2d)[0]
        proba = model.predict_proba(features_2d)[0]
        confidence = float(np.max(proba))
        label = REVERSE_LABELS[prediction]

        return label, confidence
    except Exception as e:
        print(f"Classification error: {e}")
        return None, 0.0


def benchmark(model, data_dir):
    """Benchmark model inference speed"""
    import glob
    wav_files = []
    for class_name in REVERSE_LABELS.values():
        class_dir = os.path.join(data_dir, class_name)
        if os.path.exists(class_dir):
            wav_files.extend(glob.glob(os.path.join(class_dir, "*.wav")))

    if not wav_files:
        print("No test files found")
        return

    # Warmup
    classify_file(model, wav_files[0])

    # Benchmark
    latencies = []
    for wav_file in wav_files[:50]:
        start = time.time()
        classify_file(model, wav_file)
        latencies.append((time.time() - start) * 1000)

    print(f"\nInference Benchmark (50 files):")
    print(f"  Mean latency: {np.mean(latencies):.2f} ms")
    print(f"  Median latency: {np.median(latencies):.2f} ms")
    print(f"  95th percentile: {np.percentile(latencies, 95):.2f} ms")
    print(f"  99th percentile: {np.percentile(latencies, 99):.2f} ms")


def main():
    parser = argparse.ArgumentParser(description="Evaluate AMD model")
    parser.add_argument("--model", default="ml/models/amd_model.pkl", help="Model path")
    parser.add_argument("--file", help="Single file to classify")
    parser.add_argument("--benchmark", action="store_true", help="Run benchmark")
    parser.add_argument("--data", default="ml/data", help="Test data directory")
    args = parser.parse_args()

    if not os.path.exists(args.model):
        print(f"Model not found: {args.model}")
        print("Train a model first: python ml/src/train.py --generate")
        return

    print(f"Loading model from: {args.model}")
    model = load_model(args.model)

    if args.file:
        print(f"\nClassifying: {args.file}")
        label, confidence = classify_file(model, args.file)
        print(f"  Result: {label} (confidence: {confidence:.2f})")

    if args.benchmark:
        benchmark(model, args.data)


if __name__ == "__main__":
    main()
