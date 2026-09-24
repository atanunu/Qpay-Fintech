import React from 'react';
import {createRoot} from 'react-dom/client';
import {BrowserRouter,Route,Routes,Link,useLocation} from 'react-router-dom';
import {AuthProvider,LoginPage,EnrolPage,InvitePage,AccessHelp,RecoveryPage} from './auth';
import {Layout} from './layout';
import {Overview} from './pages/dashboard';
import {RecordsPage,RecordDetail} from './pages/records';
import {PolicyPage,SecurityPage,SystemPage,ReportsPage,TemplatesPage,IdentityPage} from './pages/settings';
import './styles.css';
class Boundary extends React.Component<{children:React.ReactNode},{error:boolean}>{state={error:false};static getDerivedStateFromError(){return {error:true}}render(){return this.state.error?<main className="standalone"><h1>The workspace could not render</h1><p>No automatic mutation retry has occurred. Reload, then inspect the original record before taking another action.</p><button onClick={()=>location.reload()}>Reload workspace</button></main>:this.props.children}}
function Records(){const location=useLocation();return <RecordsPage key={location.pathname}/>}
function Record(){const location=useLocation();return <RecordDetail key={location.pathname}/>;}
function Identity(){const location=useLocation();return <IdentityPage key={location.pathname}/>;}
function App(){return <Boundary><BrowserRouter><AuthProvider><Routes><Route path="/login" element={<LoginPage/>}/><Route path="/enrol" element={<EnrolPage/>}/><Route path="/accept-invite" element={<InvitePage/>}/><Route path="/recover-access" element={<RecoveryPage/>}/><Route path="/access-help" element={<AccessHelp/>}/><Route element={<Layout/>}><Route index element={<Overview/>}/><Route path="/records/:resource" element={<Records/>}/><Route path="/records/:resource/:id" element={<Record/>}/><Route path="/policy" element={<PolicyPage/>}/><Route path="/security" element={<SecurityPage/>}/><Route path="/system" element={<SystemPage/>}/><Route path="/reports" element={<ReportsPage/>}/><Route path="/templates" element={<TemplatesPage/>}/><Route path="/identity/:id" element={<Identity/>}/><Route path="*" element={<><h1>Page not found</h1><Link to="/">Return to overview</Link></>}/></Route></Routes></AuthProvider></BrowserRouter></Boundary>}
createRoot(document.getElementById('root')!).render(<App/>);
