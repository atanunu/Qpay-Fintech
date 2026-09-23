import { Component } from 'react';
import type { ErrorInfo, ReactNode } from 'react';
import { Link, Navigate, Route, Routes, useLocation } from 'react-router-dom';
import { SessionGate } from './components/layout';
import { PageTitle } from './components/ui';
import { AuthHelpPage, AuthLayout, ForgotPage, LoginPage, RegisterPage, VerifyPage } from './pages/auth';
import { OverviewPage, WalletPage } from './pages/overview';
import { ActivityPage, PaymentDetailPage, RecoveryPage } from './pages/activity';
import { BillsPage, PaymentPage, TransferLanding } from './pages/payments';
import { BeneficiariesPage, DevicesPage, PreferencesPage, PrivacyPage, ProfilePage, SecurityPage, SettingsLayout, VerificationPage } from './pages/account';
import { NotificationsPage, NotificationDestination, StatementsPage } from './pages/records';
import { CasePage, NewCasePage, SupportPage } from './pages/support';
import { ReviewPage } from './pages/review';
class ScreenBoundary extends Component<{ children: ReactNode }, { failed: boolean }> {
  state = { failed: false };
  static getDerivedStateFromError() { return { failed: true }; }
  componentDidCatch(_error: Error, _info: ErrorInfo) { /* Do not log payment payloads, credentials or rendered customer content. */ }
  render() {
    return this.state.failed ? <main className="fatal-screen"><h1>A screen could not be displayed.</h1><p>No replacement payment will be sent. Reload the application and review any unresolved operation before making another payment.</p><button className="button primary" onClick={() => window.location.reload()}>Reload safely</button><Link className="button secondary" to="/transfers/recovery">Open payment recovery</Link></main> : this.props.children;
  }
}
function NotFound() { return <><PageTitle title="That page is not here." description="Use the account navigation or return to your overview."/><Link to="/" className="button primary">Go to overview</Link></>; }
export default function App() {
  const location = useLocation();
  return <ScreenBoundary key={location.pathname}><Routes>
    <Route element={<AuthLayout/>}><Route path="/login" element={<LoginPage/>}/><Route path="/register" element={<RegisterPage/>}/><Route path="/verify-email" element={<VerifyPage/>}/><Route path="/forgot-password" element={<ForgotPage/>}/><Route path="/reset-password" element={<VerifyPage reset/>}/><Route path="/help" element={<AuthHelpPage/>}/></Route>
    <Route element={<SessionGate/>}><Route index element={<OverviewPage/>}/><Route path="/wallet" element={<WalletPage/>}/><Route path="/transfers" element={<TransferLanding/>}/><Route path="/transfers/new" element={<PaymentPage/>}/><Route path="/transfers/recovery" element={<RecoveryPage/>}/><Route path="/bills" element={<BillsPage/>}/><Route path="/bills/pay" element={<PaymentPage bill/>}/><Route path="/activity" element={<ActivityPage/>}/><Route path="/activity/:id" element={<PaymentDetailPage/>}/><Route path="/beneficiaries" element={<BeneficiariesPage/>}/><Route path="/statements" element={<StatementsPage/>}/><Route path="/notifications" element={<NotificationsPage/>}/><Route path="/notifications/view/:reference" element={<NotificationDestination/>}/><Route path="/verification" element={<VerificationPage/>}/>
      <Route path="/settings" element={<SettingsLayout/>}><Route index element={<Navigate to="profile" replace/>}/><Route path="profile" element={<ProfilePage/>}/><Route path="security" element={<SecurityPage/>}/><Route path="devices" element={<DevicesPage/>}/><Route path="preferences" element={<PreferencesPage/>}/><Route path="privacy" element={<PrivacyPage/>}/></Route>
      <Route path="/support" element={<SupportPage/>}/><Route path="/support/new" element={<NewCasePage/>}/><Route path="/support/:id" element={<CasePage/>}/><Route path="/review" element={<ReviewPage/>}/><Route path="*" element={<NotFound/>}/>
    </Route>
  </Routes></ScreenBoundary>;
}
