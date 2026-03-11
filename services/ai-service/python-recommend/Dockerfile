FROM docker.1ms.run/library/python:3.11-slim
WORKDIR /app
COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt -i https://pypi.tuna.tsinghua.edu.cn/simple
COPY app.py ./
EXPOSE 18080
CMD ["uvicorn", "app:app", "--host", "0.0.0.0", "--port", "18080"]
