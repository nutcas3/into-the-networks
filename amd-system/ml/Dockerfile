FROM python:3.11-slim

WORKDIR /app

# Install system dependencies for librosa
RUN apt-get update && apt-get install -y \
    libsndfile1 \
    ffmpeg \
    && rm -rf /var/lib/apt/lists/*

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY src/ ./src/
COPY models/ ./models/
RUN chmod +x ./src/entrypoint.sh

ENV ML_PORT=5000
ENV MODEL_PATH=/app/models/amd_model.pkl
ENV SAMPLE_RATE=8000
ENV N_MFCC=13

EXPOSE 5000

CMD ["./src/entrypoint.sh"]
