# Qt WebEngine Flickering Test Builds Checklist

Test builds for investigating flickering on Windows with multiple QWebEngineView instances.

## Test Environment
- OS: Windows 11
- Application: Anki + Langkit addon
- Test areas: WelcomePopup, Settings panel, FeatureCard expansion

## Test Results

| Build | Description | Flickering? | Notes |
|-------|-------------|-------------|-------|
| baseline | No changes (current master) | | |
| commit 1 | test: disable GPU-accelerated canvas/WebGL to investigate flickering | | QWebEngineSettings in anki-addon |
| commit 2 | test: disable backdrop-filter in GlowEffect and Settings panel | | GlowEffect disabled + Settings panel solid bg |
| commit 3 | test: disable backdrop-filter in WelcomePopup | | Solid bg instead of blur(24px) |
| commit 4 | test: disable backdrop-filter in DependenciesChecklist | | Solid bg-black/40 on dependency cards |
| commit 5 | test: disable 3D transforms and animations in FeatureCard | | No translateZ/rotateX, no filter, no height transition |

## Flickering Scale
- None: No flickering observed
- Slight: Minor flickering, barely noticeable
- Moderate: Noticeable flickering but usable
- Severe: Heavy flickering, unusable

## Test Procedure
1. Open Anki
2. Launch Langkit addon
3. Test WelcomePopup (first launch or fresh install)
4. Test Settings panel (open/close, interact with inputs)
5. Test FeatureCard (click to expand/collapse options)
6. Note any flickering and when it occurs
