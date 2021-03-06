import os
import tempfile
import re
import shutil
import pandas as pd
import tempfile
import json as stdjson
from ludwig import api
from sanic import Sanic
from sanic.response import json

app = Sanic(name="COMPUTE_WORKER")

models = {}

def save_to_zip(input_filename, output_filename:str):
    shutil.make_archive(output_filename, 'zip', input_filename)

@app.post('/load/<key>')
async def load(request, key):
    model = request.files.get('model')
    if not model:
        return json({'status': 400, 'msg': "model not present"})
    
    metadata = request.files.get('metadata')

    if not metadata:
        return json({'status': 400, 'msg': 'metadata not present'})
    
    io_params = request.files.get('io_params')

    if not io_params:
        return json({'status': 400, 'msg': "io_params not present"})
    
    try:
        os.mkdir(f"./{key}")
    except FileExistsError:
        pass
    
    with open(f"./models/{key}/{model.name}", 'wb') as fmodel, open(f"./models/{key}/{metadata.name}", "wb") as fmeta, open(f"./models/{key}/{io_params.name}", "wb") as fio:
        fmodel.write(model.body)
        fmeta.write(metadata.body)
        fio.write(io_params.body)

    io_params_json = stdjson.loads(io_params.body)

    print(io_params_json)
    api.LudwigModel(config=io_params_json)
    return json({'msg': 'Got both'})

@app.post('/train/<key>')
async def train(request, key):
    bio_params = request.files.get('io_params')
    training_data = request.files.get('training_data')

    
    if not training_data:
        return json({'status': 400, 'msg': ""})
    

    if not bio_params:
        return json({'status': 400, 'msg': ""})
    
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
    
    save_to_zip(f"./models/{key}", f"./{directory.name}/out")

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
        return json({'status': 400, 'msg': ''})
    
    (df,_) = model.predict(request.json)
    return json({'status': 200, 'msg': df.to_dict()})

def get_latest_model(path: str):
    files = os.listdir(path)
    files.sort() # get the latest this way
    best_match = None
    for file in files:
        best_match = file
    return f"{path}/{best_match}/model"

    

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
