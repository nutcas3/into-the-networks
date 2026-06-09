# AMD Models

This directory contains trained machine learning models for Answering Machine Detection.

## Model Format

Models are serialized using Python `pickle` and trained with scikit-learn.

## Generating a Model

### Option 1: Full Pipeline (Recommended)

```bash
cd ml

# Generate synthetic training data
python src/generate_data.py --output data --samples 100

# Train the model
python src/train.py --data data --output models/amd_model.pkl --model random_forest

# Evaluate
python src/evaluate.py --model models/amd_model.pkl --benchmark
```

### Option 2: Quick Generate Script

```bash
cd ml
python src/generate_model.py --output models/amd_model.pkl
```

This generates synthetic data, trains a RandomForest classifier, and saves the model in one step.

### Option 3: Docker

```bash
docker-compose up -d amd-ml
```

The ML service will start with a dummy classifier. Train a model by exec'ing into the container:

```bash
docker exec -it amd-ml bash
python src/generate_model.py --output models/amd_model.pkl
```

Then restart the service to pick up the model.

## Model Metadata

When a model is saved, a companion JSON metadata file is created:

```json
{
  "model_type": "random_forest",
  "accuracy": 0.97,
  "classes": ["human", "answering_machine", "beep", "fax", "silence"],
  "feature_dim": 64
}
```

## Classes

| Class | Description |
|-------|-------------|
| `human` | Live person answered |
| `answering_machine` | Voicemail greeting |
| `beep` | Answering machine beep tone |
| `fax` | Fax machine tone |
| `silence` | No audio / dead air |

## Fallback Behavior

If no model is present, the ML service uses a heuristic classifier based on audio energy levels that provides reasonable accuracy for basic detection.
