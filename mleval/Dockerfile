FROM python:3.8

RUN mkdir ./models
COPY main.py ./
COPY poetry.lock ./
COPY pyproject.toml ./

RUN pip install h5py
RUN pip install poetry
RUN poetry install

CMD ["poetry", "run", "python", "main.py"]
