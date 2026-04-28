FROM python:3.12-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY app.py .

# 环境变量（可在 docker-compose 中覆盖）
ENV PORT=5000
ENV DEBUG=false
ENV SCRAPER_API_KEY=""
ENV JAVBUS_URL="https://www.javbus.com"
ENV JAVDB_URL="https://javdb.com"

EXPOSE 5000

CMD ["python", "app.py"]
