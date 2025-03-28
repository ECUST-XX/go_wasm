const input = document.getElementById('input')
const output = document.getElementById('output')

if (!WebAssembly.instantiateStreaming) { // polyfill
    WebAssembly.instantiateStreaming = async (resp, importObject) => {
        const source = await (await resp).arrayBuffer();
        return await WebAssembly.instantiate(source, importObject);
    };
}

const go = new Go();
let mod, inst;
WebAssembly.instantiateStreaming(fetch("phash.wasm"), go.importObject).then((result) => {
    mod = result.module;
    inst = result.instance;
    go.run(inst);

    // 异步返回无法直接获取
    const c =  wget();
    console.log("同步获取 c:",Uint8ArrayToString(c),c)
    setTimeout(() => {
        console.log("异步获取 c res:",Uint8ArrayToString(c))
    }, "1000");
    
}).catch((err) => {
    console.error(err);
});

input.onchange = function (event) {
  const files = event.target.files

  output.innerHTML = ''

  for(let file of files) {
    const reader = new FileReader()

    reader.onload = event => {
        const image = new Image()
        image.src = event.target.result
        output.appendChild(image)

        hash = phash(reader.result.replace(/data:[^\,]*;base64\,/,""))
        const textbox = document.createElement('div')
        textbox.innerText = `hash: ${hash}`
        output.appendChild(textbox)
    }

    reader.readAsDataURL(file)
  }

}

function Uint8ArrayToString(fileData){
    var dataString = "";
    for (var i = 0; i < fileData.length; i++) {
      dataString += String.fromCharCode(fileData[i]);
    }
   
    return dataString
}
