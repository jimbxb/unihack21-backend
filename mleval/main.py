import os
import tempfile
import re
import shutil
import pandas as pd
import zipfile
import datetime
from contextlib import closing
from zipfile import ZipFile, ZIP_DEFLATED
import json as stdjson
from ludwig import api
import psutil
from sanic import Sanic
from sanic.response import json
from sanic_cors import CORS, cross_origin

app = Sanic(name="COMPUTE_WORKER")
CORS(app, automatic_options=True)

app.config.REQUEST_MAX_SIZE = 1 << 31 -1
app.config.REQUEST_TIMEOUT = 1 << 31 -1

models = {}

latencies = []

@app.middleware('request')
async def add_start_time(request):
    request.ctx.start_time = datetime.datetime.now()

@app.middleware('response')
async def add_spent_time(request, response):
    latency = datetime.datetime.now() - request.ctx.start_time
    request.ctx.spent_time = latency
    global latencies
    latencies.append(str(latency))

def zipdir(basedir, archivename):
    assert os.path.isdir(basedir)
    with closing(ZipFile(archivename, "w", ZIP_DEFLATED)) as z:
        for root, dirs, files in os.walk(basedir):
            #NOTE: ignore empty directories
            for fn in files:
                absfn = os.path.join(root, fn)
                zfn = absfn[len(basedir)+len(os.sep):] #XXX: relative path
                z.write(absfn, zfn)

def save_to_zip(input_filename, output_filename:str):
    shutil.make_archive(output_filename, 'zip', input_filename)

@app.post('/load/<key>')
async def load(request, key):
    model = request.files.get('model')
    if not model:
        print("cant get model")
        return json({'status': 400, 'msg': "model not present"}, status=400)
    

    metadata = request.files.get('metadata')



    if not metadata:
        print("cant get metadata")
        return json({'status': 400, 'msg': 'metadata not present'}, status=400)
    
    io_params = request.files.get('io_params')


    if not io_params:
        print("cant get io_params")
        return json({'status': 400, 'msg': "io_params not present"},status=400)
    
    try:
        os.mkdir(f"./models/{key}")
    
    except FileExistsError:
        pass
    
    tempdirectory = tempfile.TemporaryDirectory()

    with open(f"{tempdirectory.name}/{model.name}", 'wb') as fmodel:
        fmodel.write(model.body)

    with zipfile.ZipFile(f"{tempdirectory.name}/{model.name}", 'r') as zip_ref:
        zip_ref.extractall(f"./models/{key}")

    load_models()

    return json({'status': 200, 'msg': 'Got both'})


@app.post('/train/<key>')
async def train(request, key):
    bio_params = request.files.get('io_params')
    training_data = request.files.get('training_data')

    
    if not training_data:
        return json({'status': 400, 'msg': ""},status=400)
    

    if not bio_params:
        return json({'status': 400, 'msg': ""}, status=400)
    
    try:
        os.mkdir(f"./models/{key}")
    except FileExistsError:
        pass
    
    io_params_json = stdjson.loads(bio_params.body)
    model = api.LudwigModel(config=io_params_json)

    with open(f"./models/{key}/{training_data.name}", "wb") as fdata:
        fdata.write(training_data.body)
    
    model.train(dataset=f"./models/{key}/{training_data.name}", output_directory=f"./models/{key}/results")

    global models
    models[key] = {"model" : model, "params" : io_params_json}

    with open(f"./models/{key}/io_params.json", "wb") as fio:
        fio.write(bio_params.body)
    
    directory = tempfile.TemporaryDirectory()
    print("SAVED TO", directory.name)

    zipdir(f"./models/{key}", f"./out.zip")

    return json({'status': 200, 'msg': "DONE"})

def load_model(path: str, key:str):
    data = None
    with open(f"{path}/io_params.json", "rb") as fm:
        data = fm.read()
    
    if not data:
        return False
    
    io_params = stdjson.loads(data)

    location = get_latest_model(f"{path}/results")
    model = api.LudwigModel.load(location)

    global models
    models[key] = {"model": model, "params": io_params}

    return True


def load_models():
    files = os.listdir("./models")
    for file in files:
        load_model(f"./models/{file}", file)

def validate_params(in_params, io_params):
    input_needed = io_params["input_features"]
    for input_dict in input_needed:
        name = input_dict["name"]
        if not in_params[name]:
            return False
    return True

@app.post('/eval/<key>')
async def eval(request, key):
    global models
    if not models[key]:
        return json({'status': '400', 'msg': "key not available"})
    
    model = models[key]

    io_params = model["params"]
    model = model["model"]

    if not validate_params(request.json, io_params):
        return json({'status': 400, 'msg': ''}, status=400)
    
    (df,_) = model.predict(request.json)
    return json({'status': 200, 'msg': df.to_dict()})

def get_latest_model(path: str):
    files = os.listdir(path)
    files.sort() # get the latest this way
    best_match = None
    for file in files:
        best_match = file
    return f"{path}/{best_match}/model"


@app.route('/stats')
async def stats(request):
    cpu_percent = psutil.cpu_percent()
    memory_percent = psutil.virtual_memory().available * 100 / psutil.virtual_memory().total
    global latencies
    return json({"time":  str( datetime.datetime.utcnow() ), "cpu_percent": cpu_percent, "memory_percent": memory_percent, "latencies": latencies})
    

@app.route('/test')
async def test(request):
    return json({'hello': 'world'})


if __name__ == "__main__":
    # location = get_latest_model("./key1/results")
    # model = api.LudwigModel.load(location)
    # out = model.predict({"doc_text": ["football"]})
    # print(out)
    load_models()
    app.run(host='0.0.0.0', port=3000)
