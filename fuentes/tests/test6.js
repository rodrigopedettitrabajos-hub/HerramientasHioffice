const { chromium } = require('playwright');
const S='/tmp/claude-0/-home-user-conciliaciones-Magma-Rodelu/2d5e639d-449d-5cd5-8193-93cf201d96da/scratchpad', U='/root/.claude/uploads/2d5e639d-449d-5cd5-8193-93cf201d96da';
const txt = (p,s) => p.$eval(s, e => e.innerText.replace(/\n+/g,' ⏎ ').slice(0,600));
(async()=>{
 const b=await chromium.launch(); const p=await b.newPage({viewport:{width:1280,height:900},acceptDownloads:true}); const errs=[]; p.on('pageerror',e=>errs.push(e.message));
 await p.goto('http://127.0.0.1:8765/'); await p.waitForTimeout(500);
 // validate tab
 await p.click('button[data-tab=validate]');
 for (const f of ['b075b626-Exportaci_n_96911_2026-11-02.json','ee654170-Dashboard_96911_2026-11-02.json','c996bf7f-Informe_Ventas_x_Art_Exportacion.json']){
   await p.setInputFiles('#fileVal', U+'/'+f); await p.waitForTimeout(200); console.log('VAL', f.slice(0,8), await txt(p,'#valOut'));
 }
 // bridge
 await p.click('button[data-tab=bridge]');
 console.log('MODE', await txt(p,'#brMode'));
 await p.click('#tab-bridge summary >> text=Avanzado'); await p.fill('#brLoginHost','http://127.0.0.1:9900'); await p.fill('#brEmail','a@b.c'); await p.fill('#brPass','mala'); await p.click('#brLogin'); await p.waitForTimeout(800);
 console.log('LOGIN bad:', await txt(p,'#brSession'));
 await p.fill('#brPass','secreto'); await p.click('#brLogin'); await p.waitForTimeout(800);
 console.log('LOGIN ok:', await txt(p,'#brSession'));
 await p.fill('#exId','20'); await p.click('#exRun'); await p.waitForSelector('#exOut .doc'); console.log('EXPORT:', await txt(p,'#exOut'));
 await (await p.$('#tab-bridge section:nth-of-type(2)')).screenshot({path:S+'/br_export.png'});
 await p.fill('#exId','999'); await p.click('#exRun'); await p.waitForTimeout(800); console.log('EXPORT bad:', await txt(p,'#exOut'));
 await p.check('input[name=exKind][value=dashboard]'); await p.fill('#exId','5e3296f1-guid'); await p.click('#exLoadFilters'); await p.waitForTimeout(800);
 console.log('FILTERS:', await txt(p,'#exFilters'));
 await p.fill('#exFilters input[data-k=value]','7'); await p.click('#exRun'); await p.waitForSelector('#exOut .doc'); console.log('EXPORT dash:', (await txt(p,'#exOut')).slice(0,300));
 // import
 await p.fill('#imId','e185589c'); await p.click('#imRun'); await p.waitForTimeout(300); console.log('IMPORT noconfirm:', await txt(p,'#imOut'));
 await p.check('#imConfirm'); await p.check('input[name=imMode][value=file]'); await p.setInputFiles('#fileIm', S+'/mock/good.csv');
 await p.click('#imRun'); await p.waitForTimeout(1500); console.log('IMPORT running:', await txt(p,'#imOut'));
 await p.waitForFunction(()=>document.getElementById('imStop').disabled, null, {timeout:20000}); console.log('IMPORT good:', await txt(p,'#imOut'));
 await p.setInputFiles('#fileIm', S+'/mock/bad.csv'); await p.click('#imRun'); await p.waitForTimeout(500);
 await p.waitForFunction(()=>document.getElementById('imStop').disabled, null, {timeout:20000}); console.log('IMPORT bad:', await txt(p,'#imOut'));
 await (await p.$('#tab-bridge section:nth-of-type(3)')).screenshot({path:S+'/br_import.png'});
 await p.check('input[name=imMode][value=body]'); await p.fill('#imBody','{"items":[1]}'); await p.click('#imRun'); await p.waitForTimeout(500);
 await p.waitForFunction(()=>document.getElementById('imStop').disabled, null, {timeout:20000}); console.log('IMPORT body:', (await txt(p,'#imOut')).slice(0,120));
 await p.click('#brLogout'); await p.waitForTimeout(500); console.log('LOGOUT:', await txt(p,'#brSession'));
 console.log('LOG has password?', (await p.$eval('#brLog', e=>e.innerText)).includes('secreto'));
 // transform still works + output validation
 await p.click('button[data-tab=transform]');
 await p.setInputFiles('#fileSrc', U+'/ee654170-Dashboard_96911_2026-11-02.json'); await p.waitForSelector('#secOpts:not(.hidden)');
 const [dl]=await Promise.all([p.waitForEvent('download'),p.click('#btnGo')]); console.log('TRANSFORM:', (await txt(p,'#result')).slice(0,400));
 console.log('errors',errs); await b.close();
})();
