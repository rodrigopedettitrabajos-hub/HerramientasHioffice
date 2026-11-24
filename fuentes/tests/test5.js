const { chromium } = require('playwright');
const S='/tmp/claude-0/-home-user-conciliaciones-Magma-Rodelu/2d5e639d-449d-5cd5-8193-93cf201d96da/scratchpad', U='/root/.claude/uploads/2d5e639d-449d-5cd5-8193-93cf201d96da';
const heads = p => p.$eval('#pvTable thead tr.h',e=>[...e.querySelectorAll('th')].map(t=>t.innerText.trim()).filter(Boolean).join(' | '));
(async()=>{
 const b=await chromium.launch(); const ctx=await b.newContext({viewport:{width:1280,height:900},acceptDownloads:true}); const p=await ctx.newPage(); const errs=[]; p.on('pageerror',e=>errs.push(e.message));
 await p.goto('file://'+S+'/Conversor_Dashboard_Exportacion.html');
 await p.setInputFiles('#fileSrc', U+'/c996bf7f-Informe_Ventas_x_Art_Exportacion.json'); await p.waitForSelector('#secPrev:not(.hidden)');
 console.log('ventas:', await heads(p));
 await (await p.$('#secPrev')).screenshot({path:S+'/pv2_ventas.png'});
 await p.setInputFiles('#fileSrc', U+'/b075b626-Exportaci_n_96911_2026-11-02.json'); await p.waitForSelector('#secPrev:not(.hidden)');
 console.log('facturas:', await heads(p));
 // edit a name in configurator
 await p.fill('#cfgTable input[data-key="attr:1660"]', 'Serie'); await p.press('#cfgTable input[data-key="attr:1660"]','Enter');
 console.log('facturas after edit:', await heads(p));
 await (await p.$('#secNames')).screenshot({path:S+'/cfg.png'});
 // pristine bake
 const [dl]=await Promise.all([p.waitForEvent('download'),p.click('#btnBakeNames')]); await dl.saveAs(S+'/baked.html');
 // conversion still works after preview
 const [dl2]=await Promise.all([p.waitForEvent('download'),p.click('#btnGo')]); console.log(await p.$eval('#result',e=>e.innerText));
 // fresh context (no localStorage) with baked copy
 const ctx2=await b.newContext(); const p2=await ctx2.newPage(); p2.on('pageerror',e=>errs.push('baked: '+e.message));
 await p2.goto('file://'+S+'/baked.html');
 await p2.setInputFiles('#fileSrc', U+'/b075b626-Exportaci_n_96911_2026-11-02.json'); await p2.waitForSelector('#secPrev:not(.hidden)');
 console.log('baked facturas:', await heads(p2));
 console.log('errors',errs); await b.close();
})();
