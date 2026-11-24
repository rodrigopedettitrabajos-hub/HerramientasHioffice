const { chromium } = require('playwright'); const fs=require('fs');
const S='/tmp/claude-0/-home-user-conciliaciones-Magma-Rodelu/2d5e639d-449d-5cd5-8193-93cf201d96da/scratchpad', U='/root/.claude/uploads/2d5e639d-449d-5cd5-8193-93cf201d96da';
const txt = (p,s) => p.$eval(s, e => e.innerText.replace(/\n+/g,' ⏎ ').slice(0,300));
(async()=>{
 const b=await chromium.launch(); const errs=[];
 let p=await b.newPage({viewport:{width:1280,height:900}}); p.on('pageerror',e=>errs.push(e.message));
 await p.goto('http://127.0.0.1:8765/'); await p.waitForSelector('#cfgModal:not(.hidden)');
 console.log('FIRST RUN modal:', await txt(p,'#cfIntro'), '| remember visible?', await p.isVisible('#cfRemember'));
 await p.screenshot({path:S+'/cfg_modal.png'});
 await p.fill('#cfEmail','rodri@test.uy'); await p.click('#cfgModal summary'); await p.fill('#cfHost','http://127.0.0.1:9900'); await p.fill('#cfPass','secreto');
 await p.click('#cfSave'); await p.waitForTimeout(1500);
 console.log('AFTER SAVE: tab bridge?', await p.isVisible('#tab-bridge'), '|', await txt(p,'#brSession'));
 // name edit -> file
 await p.click('button[data-tab=transform]'); await p.setInputFiles('#fileSrc', U+'/b075b626-Exportaci_n_96911_2026-11-02.json'); await p.waitForSelector('#secNames:not(.hidden)');
 await p.fill('#cfgTable input[data-key="attr:1660"]','Serie'); await p.press('#cfgTable input[data-key="attr:1660"]','Enter'); await p.waitForTimeout(800);
 console.log('FILES:', fs.readdirSync(S+'/exetest/datos').join(', '));
 console.log('config.json:', fs.readFileSync(S+'/exetest/datos/configuracion.json','utf8').replace(/\s+/g,' '));
 console.log('nombres:', fs.readFileSync(S+'/exetest/datos/nombres_columnas.json','utf8').replace(/\s+/g,' '));
 await p.close();
 // new browser context (empty localStorage): config + names come from files, no modal
 const ctx2=await b.newContext({viewport:{width:1280,height:900}}); const p2=await ctx2.newPage(); p2.on('pageerror',e=>errs.push(e.message));
 await p2.goto('http://127.0.0.1:8765/'); await p2.waitForTimeout(1200);
 console.log('SECOND RUN modal hidden?', await p2.isHidden('#cfgModal'), '| email prefilled:', await p2.inputValue('#brEmail'), '| config btn:', await p2.isVisible('#btnConfig'));
 await p2.click('button[data-tab=transform]'); await p2.setInputFiles('#fileSrc', U+'/b075b626-Exportaci_n_96911_2026-11-02.json'); await p2.waitForSelector('#secPrev:not(.hidden)');
 console.log('NAME from file:', (await p2.$eval('#pvTable thead tr.h',e=>e.innerText)).split('\n').slice(0,2).join(' | '));
 await p2.click('button[data-tab=bridge]'); await p2.fill('#brPass','secreto'); await p2.click('#brLogin'); await p2.waitForTimeout(800); console.log('LOGIN:', await txt(p2,'#brSession'));
 await p2.fill('#exId','20'); await p2.click('#exRun'); await p2.waitForSelector('#exOut .doc'); console.log('EXPORT:', (await txt(p2,'#exOut')).slice(0,90));
 await p2.click('#btnConfig'); console.log('CONFIG reopen visible:', await p2.isVisible('#cfgModal')); await p2.click('#cfCancel');
 await p2.click('#btnQuit'); await p2.waitForTimeout(800); console.log('QUIT page:', await txt(p2,'body'));
 console.log('errors',errs); await b.close();
})();
