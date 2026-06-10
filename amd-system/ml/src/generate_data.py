"""
Generate synthetic audio training data for Answering Machine Detection.
Creates wav files simulating: human speech, answering machine greetings, beeps, fax tones, silence.
"""
import os
import argparse
import numpy as np
from scipy.io import wavfile


def generate_sine_wave(freq, duration, sample_rate=8000, amplitude=0.5):
    """Generate a pure sine wave"""
    t = np.linspace(0, duration, int(sample_rate * duration), endpoint=False)
    return (amplitude * np.sin(2 * np.pi * freq * t)).astype(np.float32)


def generate_noise(duration, sample_rate=8000, amplitude=0.1):
    """Generate random noise"""
    samples = int(sample_rate * duration)
    return (amplitude * np.random.randn(samples)).astype(np.float32)


def generate_human_speech(duration=3.0, sample_rate=8000):
    """
    Generate synthetic 'speech' using multiple modulated sine waves
    to simulate human voice frequency characteristics.
    """
    t = np.linspace(0, duration, int(sample_rate * duration), endpoint=False)
    signal = np.zeros_like(t, dtype=np.float32)

    # Fundamental frequency (fundamental voice freq ~150-250Hz)
    fund_freq = np.random.uniform(150, 250)
    signal += 0.4 * np.sin(2 * np.pi * fund_freq * t)

    # Formants (vocal tract resonances)
    formants = [np.random.uniform(500, 1000),
                np.random.uniform(1000, 2000),
                np.random.uniform(2000, 3500)]
    for f in formants:
        signal += 0.2 * np.sin(2 * np.pi * f * t)

    # Amplitude modulation (simulates syllables)
    mod_freq = np.random.uniform(3, 8)
    envelope = 0.5 + 0.5 * np.sin(2 * np.pi * mod_freq * t)
    signal *= envelope

    # Add slight noise
    signal += generate_noise(duration, sample_rate, amplitude=0.05)

    return signal.astype(np.float32)


def generate_answering_machine(duration=4.0, sample_rate=8000):
    """
    Generate synthetic answering machine greeting.
    Similar to speech but more monotone and with pause patterns.
    """
    t = np.linspace(0, duration, int(sample_rate * duration), endpoint=False)
    signal = np.zeros_like(t, dtype=np.float32)

    # More monotone voice
    fund_freq = np.random.uniform(180, 220)
    signal += 0.5 * np.sin(2 * np.pi * fund_freq * t)

    # Fewer formants, more stable
    formants = [700, 1400, 2600]
    for f in formants:
        signal += 0.15 * np.sin(2 * np.pi * f * t)

    # Slower modulation (more monotone)
    mod_freq = np.random.uniform(1.5, 3)
    envelope = 0.5 + 0.5 * np.sin(2 * np.pi * mod_freq * t)
    signal *= envelope

    signal += generate_noise(duration, sample_rate, amplitude=0.03)

    return signal.astype(np.float32)


def generate_beep(duration=2.0, sample_rate=8000):
    """
    Generate answering machine beep tone.
    Typical beep: 1000-2000Hz tone burst with silence.
    """
    total_samples = int(sample_rate * duration)
    signal = np.zeros(total_samples, dtype=np.float32)

    # Beep at ~1.5kHz for ~0.5 seconds
    beep_freq = np.random.uniform(1000, 2000)
    beep_duration = np.random.uniform(0.3, 0.8)
    beep_samples = int(sample_rate * beep_duration)

    t = np.linspace(0, beep_duration, beep_samples, endpoint=False)
    beep = 0.7 * np.sin(2 * np.pi * beep_freq * t)

    # Place beep somewhere in the middle
    start_idx = total_samples // 4
    signal[start_idx:start_idx + beep_samples] = beep.astype(np.float32)

    # Add faint background noise
    signal += generate_noise(duration, sample_rate, amplitude=0.02)

    return signal


def generate_fax_tone(duration=3.0, sample_rate=8000):
    """
    Generate fax CNG tone (1100Hz) or CED tone (2100Hz).
    Fax answering tone: 2100Hz every 2.6s for 2.6s
    """
    total_samples = int(sample_rate * duration)
    signal = np.zeros(total_samples, dtype=np.float32)

    # Fax CED tone at 2100Hz
    fax_freq = 2100
    tone_duration = 2.6
    tone_samples = int(sample_rate * tone_duration)

    t = np.linspace(0, tone_duration, tone_samples, endpoint=False)
    tone = 0.6 * np.sin(2 * np.pi * fax_freq * t)

    start_idx = total_samples // 4
    end_idx = min(start_idx + tone_samples, total_samples)
    signal[start_idx:end_idx] = tone[:end_idx - start_idx].astype(np.float32)

    signal += generate_noise(duration, sample_rate, amplitude=0.02)

    return signal


def generate_silence(duration=2.0, sample_rate=8000):
    """Generate near-silent audio with minimal noise"""
    return generate_noise(duration, sample_rate, amplitude=0.02)


def save_wav(signal, path, sample_rate=8000):
    """Save signal as 16-bit WAV file"""
    # Normalize and convert to int16
    max_val = np.max(np.abs(signal))
    if max_val > 0:
        signal = signal / max_val * 0.9  # Leave some headroom
    int_signal = (signal * 32767).astype(np.int16)
    wavfile.write(path, sample_rate, int_signal)


def generate_dataset(output_dir, samples_per_class=100):
    """Generate a complete synthetic dataset"""
    classes = {
        "human": generate_human_speech,
        "answering_machine": generate_answering_machine,
        "beep": generate_beep,
        "fax": generate_fax_tone,
        "silence": generate_silence,
    }

    for class_name, generator in classes.items():
        class_dir = os.path.join(output_dir, class_name)
        os.makedirs(class_dir, exist_ok=True)

        for i in range(samples_per_class):
            duration = np.random.uniform(1.5, 4.0)
            signal = generator(duration=duration)

            filename = f"{class_name}_{i:04d}.wav"
            filepath = os.path.join(class_dir, filename)
            save_wav(signal, filepath)

        print(f"Generated {samples_per_class} samples for class: {class_name}")

    print(f"Dataset generated in: {output_dir}")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Generate synthetic AMD training data")
    parser.add_argument("--output", default="ml/data", help="Output directory")
    parser.add_argument("--samples", type=int, default=100, help="Samples per class")
    args = parser.parse_args()

    generate_dataset(args.output, args.samples)
