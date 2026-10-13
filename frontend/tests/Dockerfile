FROM mcr.microsoft.com/playwright:v1.56.1-noble
ARG TARGETARCH
RUN apt-get update -qq && apt-get install -y -qq --no-install-recommends curl unzip && rm -rf /var/lib/apt/lists/*
# Chrome for Testing incluye los codecs de las clases HLS. El Chromium de
# Playwright no los incluye; no sirve para verificar reproducción real.
RUN case "$TARGETARCH" in arm64) distribution=linux-arm64 ;; *) distribution=linux64 ;; esac \
    && curl -fsSL "https://storage.googleapis.com/chrome-for-testing-public/155.0.8059.39/${distribution}/chrome-${distribution}.zip" -o /tmp/chrome.zip \
    && unzip -q /tmp/chrome.zip -d /opt \
    && mv "/opt/chrome-${distribution}" /opt/chrome \
    && rm /tmp/chrome.zip
ENV CHROME_PATH=/opt/chrome/chrome
