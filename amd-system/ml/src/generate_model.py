"""
One-shot model generation script.
Generates synthetic training data, trains a classifier, and saves the model.
"""
import os
import sys
import argparse
import tempfile
import shutil

# Add src to path for imports
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from generate_data import generate_dataset
from train import load_dataset, train_model, save_model


def main():
    parser = argparse.ArgumentParser(description="Generate AMD model in one step")
    parser.add_argument("--output", default="ml/models/amd_model.pkl", help="Output model path")
    parser.add_argument("--samples", type=int, default=100, help="Samples per class")
    parser.add_argument("--model", default="random_forest", choices=["random_forest", "logistic_regression"], help="Model type")
    parser.add_argument("--cleanup", action="store_true", help="Remove training data after training")
    args = parser.parse_args()

    # Create temporary or specified data directory
    if args.cleanup:
        data_dir = tempfile.mkdtemp(prefix="amd_data_")
    else:
        data_dir = "ml/data"
        os.makedirs(data_dir, exist_ok=True)

    try:
        print("=" * 50)
        print("AMD Model Generator")
        print("=" * 50)

        # Step 1: Generate data
        print("\n[1/3] Generating synthetic training data...")
        generate_dataset(data_dir, args.samples)

        # Step 2: Load and train
        print("\n[2/3] Training model...")
        X, y, _ = load_dataset(data_dir)

        if len(X) == 0:
            print("Error: No training data!")
            sys.exit(1)

        print(f"Dataset: {len(X)} samples, {X.shape[1]} features")
        model, accuracy = train_model(X, y, args.model)

        # Step 3: Save
        print("\n[3/3] Saving model...")
        os.makedirs(os.path.dirname(args.output), exist_ok=True)
        save_model(model, args.output, args.model, accuracy)

        print("\n" + "=" * 50)
        print(f"Model saved to: {args.output}")
        print(f"Accuracy: {accuracy:.4f}")
        print("=" * 50)

    finally:
        if args.cleanup and os.path.exists(data_dir):
            print(f"\nCleaning up temporary data: {data_dir}")
            shutil.rmtree(data_dir)


if __name__ == "__main__":
    main()
