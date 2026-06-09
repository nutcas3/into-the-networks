"""
AMD Model Training Pipeline
Trains an audio classifier for Answering Machine Detection using MFCC features.
"""
import os
import sys
import argparse
import glob
import pickle
import json

import numpy as np
import librosa
from sklearn.ensemble import RandomForestClassifier
from sklearn.linear_model import LogisticRegression
from sklearn.model_selection import train_test_split, cross_val_score
from sklearn.metrics import classification_report, confusion_matrix, accuracy_score


# Class label mapping
LABELS = {
    "human": 0,
    "answering_machine": 1,
    "beep": 2,
    "fax": 3,
    "silence": 4,
}

REVERSE_LABELS = {v: k for k, v in LABELS.items()}


def extract_features(audio_path, sample_rate=8000, n_mfcc=13):
    """Extract MFCC features from an audio file"""
    try:
        y, sr = librosa.load(audio_path, sr=sample_rate, mono=True)
    except Exception as e:
        print(f"Error loading {audio_path}: {e}")
        return None

    if len(y) == 0:
        return None

    # Extract MFCC features
    mfcc = librosa.feature.melspectrogram(
        y=y,
        sr=sr,
        n_mels=n_mfcc,
        n_fft=512,
        hop_length=256,
    )

    # Statistical features across time axis
    mfcc_mean = np.mean(mfcc, axis=1)
    mfcc_std = np.std(mfcc, axis=1)
    mfcc_max = np.max(mfcc, axis=1)
    mfcc_min = np.min(mfcc, axis=1)

    # Additional spectral features
    spectral_centroid = librosa.feature.spectral_centroid(y=y, sr=sr)[0]
    spectral_rolloff = librosa.feature.spectral_rolloff(y=y, sr=sr)[0]
    zero_crossing_rate = librosa.feature.zero_crossing_rate(y)[0]

    features = np.concatenate([
        mfcc_mean,
        mfcc_std,
        mfcc_max,
        mfcc_min,
        [np.mean(spectral_centroid), np.std(spectral_centroid)],
        [np.mean(spectral_rolloff), np.std(spectral_rolloff)],
        [np.mean(zero_crossing_rate), np.std(zero_crossing_rate)],
    ])

    return features


def load_dataset(data_dir):
    """Load all audio files and extract features"""
    X = []
    y = []
    file_paths = []

    print(f"Loading dataset from: {data_dir}")

    for class_name, label in LABELS.items():
        class_dir = os.path.join(data_dir, class_name)
        if not os.path.exists(class_dir):
            print(f"Warning: Directory not found: {class_dir}")
            continue

        wav_files = glob.glob(os.path.join(class_dir, "*.wav"))
        print(f"  {class_name}: {len(wav_files)} files")

        for wav_file in wav_files:
            features = extract_features(wav_file)
            if features is not None:
                X.append(features)
                y.append(label)
                file_paths.append(wav_file)

    return np.array(X), np.array(y), file_paths


def train_model(X, y, model_type="random_forest"):
    """Train a classification model"""
    # Split dataset
    X_train, X_test, y_train, y_test = train_test_split(
        X, y, test_size=0.2, random_state=42, stratify=y
    )

    # Select model
    if model_type == "random_forest":
        model = RandomForestClassifier(
            n_estimators=100,
            max_depth=15,
            random_state=42,
            n_jobs=-1,
        )
    elif model_type == "logistic_regression":
        model = LogisticRegression(
            max_iter=1000,
            random_state=42,
            multi_class="multinomial",
        )
    else:
        raise ValueError(f"Unknown model type: {model_type}")

    print(f"\nTraining {model_type} model...")
    model.fit(X_train, y_train)

    # Evaluate
    y_pred = model.predict(X_test)
    accuracy = accuracy_score(y_test, y_pred)

    print(f"\nTest Accuracy: {accuracy:.4f}")

    # Cross-validation
    cv_scores = cross_val_score(model, X, y, cv=5, scoring="accuracy")
    print(f"Cross-validation: {cv_scores.mean():.4f} (+/- {cv_scores.std() * 2:.4f})")

    # Classification report
    print("\nClassification Report:")
    target_names = [REVERSE_LABELS[i] for i in sorted(REVERSE_LABELS.keys())]
    print(classification_report(y_test, y_pred, target_names=target_names))

    # Confusion matrix
    print("\nConfusion Matrix:")
    cm = confusion_matrix(y_test, y_pred)
    print(cm)

    return model, accuracy


def save_model(model, output_path, model_type, accuracy):
    """Save trained model and metadata"""
    # Save pickled model
    os.makedirs(os.path.dirname(output_path), exist_ok=True)

    with open(output_path, "wb") as f:
        pickle.dump(model, f)

    # Save metadata
    metadata = {
        "model_type": model_type,
        "accuracy": accuracy,
        "classes": list(LABELS.keys()),
        "feature_dim": model.n_features_in_ if hasattr(model, "n_features_in_") else None,
    }

    meta_path = output_path.replace(".pkl", "_metadata.json")
    with open(meta_path, "w") as f:
        json.dump(metadata, f, indent=2)

    print(f"\nModel saved to: {output_path}")
    print(f"Metadata saved to: {meta_path}")


def main():
    parser = argparse.ArgumentParser(description="Train AMD classification model")
    parser.add_argument("--data", default="ml/data", help="Training data directory")
    parser.add_argument("--output", default="ml/models/amd_model.pkl", help="Output model path")
    parser.add_argument("--model", default="random_forest", choices=["random_forest", "logistic_regression"], help="Model type")
    parser.add_argument("--generate", action="store_true", help="Generate synthetic data first")
    parser.add_argument("--samples", type=int, default=100, help="Samples per class for generation")
    args = parser.parse_args()

    # Generate data if requested
    if args.generate:
        print("Generating synthetic training data...")
        from generate_data import generate_dataset
        generate_dataset(args.data, args.samples)

    # Load dataset
    X, y, _ = load_dataset(args.data)

    if len(X) == 0:
        print("Error: No training data found!")
        print(f"Run with --generate to create synthetic data, or place wav files in {args.data}/")
        sys.exit(1)

    print(f"\nTotal samples: {len(X)}")
    print(f"Feature dimensions: {X.shape[1]}")
    print(f"Class distribution: {dict(zip(*np.unique(y, return_counts=True)))}")

    # Train model
    model, accuracy = train_model(X, y, args.model)

    # Save model
    save_model(model, args.output, args.model, accuracy)


if __name__ == "__main__":
    main()
