# A headless browser the agent can drive (T-004). Approved by the user on 2026-11-22.
# Microsoft's Playwright image holds the browsers; the Python package drives them.
FROM mcr.microsoft.com/playwright/python:v1.49.0-noble
RUN pip install --no-cache-dir --break-system-packages playwright==1.49.0
WORKDIR /repo
