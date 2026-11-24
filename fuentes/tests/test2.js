const { chromium } = require('playwright');
const S='/tmp/claude-0/-home-user-conciliaciones-Magma-Rodelu/2d5e639d-449d-5cd5-8193-93cf201d96da/scratchpad', U='/root/.claude/uploads/2d5e639d-449d-5cd5-8193-93cf201d96da', H='file://'+S+'/Conversor_Dashboard_Exportacion.html';
async function run(p, src, out, tweak){
  await p.setInputFiles('#fileSrc', src); await p.waitForSelector('#secOpts:not(.hidden)');
  console.log('---', src.split('/').pop()); console.log(await p.$eval('#srcInfo',e=>e.innerText));
  if (tweak) await tweak();
  const [dl]=await Promise.all([p.waitForEvent('download'),p.click('#btnGo')]); await dl.saveAs(out);
  console.log(dl.suggestedFilename()); console.log(await p.$eval('#result',e=>e.innerText));
}
(async()=>{
 const b=await chromium.launch(); const p=await b.newPage({acceptDownloads:true}); const errs=[]; p.on('pageerror',e=>errs.push(e.message));
 await p.goto(H);
 await run(p, U+'/ee654170-Dashboard_96911_2026-11-02.json', S+'/t_exp.json');           // dashboard -> export
 await run(p, U+'/b075b626-Exportaci_n_96911_2026-11-02.json', S+'/t_rep.json', async()=>{ await p.screenshot({path:S+'/shot2.png',fullPage:true}); }); // export -> report
 await run(p, S+'/t_exp.json', S+'/t_roundtrip.json');                                   // ida y vuelta
 console.log('errors',errs); await b.close();
})();
