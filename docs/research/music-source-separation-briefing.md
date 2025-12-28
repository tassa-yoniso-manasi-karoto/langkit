# Music Source Separation: State of the Art (December 2025)

## Context for This Document

You (Claude Code) are working on Langkit, a language learning tool with a "Voice Enhancing" feature that isolates vocals from audio using deep learning. The codebase currently supports Demucs via Docker. This document briefs you on why Demucs is now outdated and what to use instead.

---

## Demucs Is Obsolete

Meta archived the Demucs repository in January 2025. While functional, it has been surpassed by newer architectures that deliver **~40% better vocal separation quality**.

| Model | Vocal SDR | Status |
|-------|-----------|--------|
| Demucs v4 | ~9.0 dB | Archived, outdated |
| BS-RoFormer | ~12.97 dB | Current standard |
| Mel-Band RoFormer | ~12.6 dB | Current standard, often preferred |

---

## The RoFormer Models

### Architecture Origin

ByteDance researchers (Wei-Tsung Lu, Ju-Chiang Wang, Qiuqiang Kong) published papers in late 2023 describing the BS-RoFormer and Mel-Band RoFormer architectures. **However, ByteDance did not release trained models or production implementations.** The actual usable models were trained and released by the community.

### BS-RoFormer (Band-Split Rotary Position Embedding Transformer)

- Won the SDX23 Music Demixing Challenge
- Splits spectrogram into frequency subbands, processes with hierarchical transformers
- Uses rotary position embeddings instead of learned absolute positions
- Best known checkpoint: `model_bs_roformer_ep_317_sdr_12.9755.ckpt` (trained by viperx)

### Mel-Band RoFormer

- Variant that uses mel-scale frequency mapping with overlapping subbands
- Better aligns with human auditory perception
- Community consensus: slightly better perceptual quality for vocals
- 97% selection rate for vocals in ensemble benchmarks
- Key community trainers: Kimberley Jensen, aufr33, viperx
- Notable checkpoints: `mel_band_roformer_kim_ft_unwa.ckpt`, `melband_roformer_instvoc_duality_v1/v2.ckpt`

---

## The `audio-separator` Library

**This is the recommended integration path for Langkit.**

GitHub: https://github.com/nomadkaraoke/python-audio-separator   (THIS REPO WAS CLONED AND IS ACCESSIBLE FOR CONSULTATION AT ./python-audio-separator)
PyPI: `audio-separator`  
License: MIT  
Stars: ~1000  
Last updated: November 2025 (v0.40.0)

### What It Does

- Unified Python library/CLI for all major separation architectures
- Supports: BS-RoFormer, Mel-Band RoFormer, MDX-Net, Demucs variants, VR Arch
- Auto-downloads model weights on first use
- Works with CUDA, CoreML (Apple Silicon), or CPU

### CLI Usage

```bash
# Install
pip install "audio-separator[gpu]"

# Separate with BS-RoFormer (best overall)
audio-separator input.wav --model_filename model_bs_roformer_ep_317_sdr_12.9755.ckpt

# Separate with Mel-Band RoFormer
audio-separator input.wav --model_filename mel_band_roformer_kim_ft_unwa.ckpt
```

### Docker Image

The library provides an official Docker image which was pulled by the user already.

Docker Hub: https://hub.docker.com/r/beveradb/audio-separator

### Python API

```python
from audio_separator.separator import Separator

separator = Separator()
separator.load_model(model_filename="model_bs_roformer_ep_317_sdr_12.9755.ckpt")
output_files = separator.separate("/path/to/audio.wav")
# Returns list of paths: [vocals_path, instrumental_path]
```

---

## Model Selection Guide

For Langkit's voice enhancing feature (isolating dialogue from music/effects):

| Use Case | Recommended Model |
|----------|-------------------|
| Best quality (default) | `mel_band_roformer_kim_ft_unwa.ckpt` or `model_bs_roformer_ep_317_sdr_12.9755.ckpt` |
| Faster/lighter | Keep Demucs as fallback option |
| Sparse vocals (lots of silence) | Mamba-based models (emerging, less mature) |

Both BS-RoFormer and Mel-Band RoFormer produce excellent results. Community preference leans slightly toward Mel-Band RoFormer for vocals specifically.

---

## Emerging Architectures (Watch List)

These are newer but less battle-tested:

### Mamba/SSM-based Models

- **TS-BSmamba2**: First Mamba-2 application to music separation (September 2024)
- **MSNet**: Published in Nature Scientific Reports 2025, claims SOTA on MUSDB18
- **"Mamba2 Meets Silence"**: Reports 11.03 dB cSDR, specifically targets intermittent vocals
- Advantage: Linear complexity vs. transformers' quadratic
- Status: Promising but community-trained weights not as mature as RoFormer

Repository for training/experimenting: https://github.com/ZFTurbo/Music-Source-Separation-Training

### SCNet (Sparse Compression Network)

- 48% less compute than Demucs
- Better for bass/drums than vocals
- GitHub: https://github.com/starrytong/SCNet

---

## Implementation Recommendation for Langkit

1. **Add `audio-separator` Docker support** alongside existing Demucs
2. **Default to Mel-Band RoFormer** (`mel_band_roformer_kim_ft_unwa.ckpt`)
3. **Expose model selection** to users who want to experiment
4. **Keep Demucs as legacy option** for users who already have it set up

The `audio-separator` Docker image is the cleanest path—it handles model downloads, GPU detection, and provides a consistent interface across all supported architectures.

---

## Key Links

| Resource | URL |
|----------|-----|
| audio-separator GitHub | https://github.com/nomadkaraoke/python-audio-separator |
| audio-separator Docker | https://hub.docker.com/r/beveradb/audio-separator |
| UVR5 (GUI alternative) | https://github.com/Anjok07/ultimatevocalremovergui |
| ZFTurbo training repo | https://github.com/ZFTurbo/Music-Source-Separation-Training |
| Model weights hub | Downloaded automatically by audio-separator |

---
