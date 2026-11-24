# Servidor simulado del Bridge según la documentación (sólo para pruebas).
import json, base64, time, uuid
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import urlsplit, parse_qs
TOK='tok-123'; RUNS={}
CSV='Cód. Artículo;Establecimiento;Artículo;Uds.V;Total\r\n540;RESISTANCE CARRASCO TEST;Medialuna;2455;261618,50\r\n80;RESISTANCE PUNTA CARRETAS TEST;CROISSANT;2324;293969,00\r\n'.encode('utf-8')
class H(BaseHTTPRequestHandler):
    def log_message(self,*a): pass
    def send(self,code,obj,ct='application/json'):
        b=obj if isinstance(obj,bytes) else (obj if isinstance(obj,str) else json.dumps(obj)).encode()
        self.send_response(code); self.send_header('Content-Type',ct); self.send_header('Content-Length',str(len(b))); self.end_headers(); self.wfile.write(b)
    def auth(self): return self.headers.get('x-auth-token')==TOK
    def do_GET(self):
        p=urlsplit(self.path); q=parse_qs(p.query)
        if p.path=='/services/cloud/getCustomerWithAuthToken':
            if q.get('password',[''])[0]!='secreto': return self.send(200,'<response><error>bad</error></response>','application/xml')
            return self.send(200,'<response><customerWithAuthTokenResponse><address>127.0.0.1</address><authToken>%s</authToken><customerId>35919</customerId><port>9900</port><secure>false</secure></customerWithAuthTokenResponse></response>'%TOK,'application/xml')
        if not self.auth(): return self.send(401,{'message':'no auth'})
        if p.path=='/ErpCloud/session/logout': return self.send(200,{})
        parts=p.path.split('/')
        if p.path.startswith('/bridge-back/api/import/'):
            u=parts[4]; r=RUNS.get(u)
            if not r: return self.send(404,{'message':'no existe'})
            done=time.time()-r['t']>3
            if parts[5]=='status':
                sid=1 if not done else (2 if r['bad'] else 3)
                return self.send(200,{'uuid':u,'startTimestamp':1609168900000,'endTimestamp':1609168905000,'recordsInserted':0 if r['bad'] else 4,'recordsUpdated':1,'filesToProcess':1,'filesProcessed':1 if done else 0,'errors':1 if (done and r['bad']) else 0,'status':{1:'En ejecución',2:'Terminado con errores',3:'Terminado'}[sid],'statusId':sid,'filesRecordsProcessed':5})
            if parts[5]=='errors': return self.send(200,{'uuid':u,'errors':[{'errorCode':'','errorText':'exception.StepException: Formato decimal no válido: 12,3,4','errorTimestamp':1609169047000}]})
        self.send(404,{'message':'?'})
    def do_POST(self):
        p=urlsplit(self.path); n=int(self.headers.get('Content-Length') or 0); body=self.rfile.read(n)
        if not self.auth(): return self.send(401,{'message':'no auth'})
        if p.path=='/ErpCloud/exportation/launch':
            j=json.loads(body)
            if j['exportationId']==999: return self.send(500,{'message':'Exportation not found'})
            return self.send(200,{'exportedDocs':[{'name':'Exportacion-%s.csv'%j['startDate'],'data':base64.b64encode(CSV).decode(),'type':1},{'name':'x.json','data':base64.b64encode(json.dumps({'filters':j['filters']}).encode()).decode(),'type':4}]})
        if p.path.startswith('/ErpCloud/exportation/getExportationDashboardFilters/'):
            return self.send(200,[{'attributeId':1658,'arithmeticOperator':'EQUAL','type':'Integer','value':'','value2':''}])
        if p.path.startswith('/bridge-back/api/import/'):
            parts=p.path.split('/'); kind=parts[5]; ct=self.headers.get('Content-Type','')
            bad = (kind=='launchWithFile' and b'12,3,4' in body) or (kind=='launchWithBody' and b'"bad"' in body)
            if kind=='launchWithFile' and 'multipart/form-data' not in ct: return self.send(400,{'message':'need multipart'})
            u=str(uuid.uuid4()); RUNS[u]={'t':time.time(),'bad':bad}
            return self.send(200,[{'configurationName':'Articulos Basic','configurationUUID':'e185589c','importUUID':u}])
        self.send(404,{'message':'?'})
ThreadingHTTPServer(('127.0.0.1',9900),H).serve_forever()
