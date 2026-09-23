/** Native WebAuthn encoding only. The maintained Go library validates cryptography. */
export function decodeBase64URL(value:string):ArrayBuffer {const text=value.replaceAll('-','+').replaceAll('_','/');const raw=atob(text+'='.repeat((4-text.length%4)%4));return Uint8Array.from(raw,c=>c.charCodeAt(0)).buffer;}
export function encodeBase64URL(value:ArrayBuffer):string {const bytes=new Uint8Array(value);let raw='';for(const byte of bytes)raw+=String.fromCharCode(byte);return btoa(raw).replaceAll('+','-').replaceAll('/','_').replace(/=+$/,'');}
export async function browserCredential(options:unknown,register:boolean):Promise<unknown>{
 if(!window.isSecureContext||!navigator.credentials||typeof PublicKeyCredential==='undefined')throw new Error('This browser needs a secure context and passkey support. Use password sign-in instead.');
 const envelope=options as {publicKey:Record<string,unknown>};const p=structuredClone(envelope.publicKey);if(!p||typeof p.challenge!=='string')throw new Error('Passkey options were not valid.');p.challenge=decodeBase64URL(p.challenge);
 for(const key of ['excludeCredentials','allowCredentials'])if(Array.isArray(p[key]))p[key]=(p[key] as {id:string}[]).map(v=>({...v,id:decodeBase64URL(v.id)}));
 if(register){const user=p.user as {id:string};p.user={...user,id:decodeBase64URL(user.id)}}
 const credential=await (register?navigator.credentials.create({publicKey:p as unknown as PublicKeyCredentialCreationOptions}):navigator.credentials.get({publicKey:p as unknown as PublicKeyCredentialRequestOptions})) as PublicKeyCredential|null;
 if(!credential)throw new Error('The passkey operation was cancelled.');
 const result:Record<string,unknown>={id:credential.id,rawId:encodeBase64URL(credential.rawId),type:credential.type,authenticatorAttachment:credential.authenticatorAttachment,clientExtensionResults:credential.getClientExtensionResults()};
 const response=credential.response;
 if(register){const v=response as AuthenticatorAttestationResponse;result.response={clientDataJSON:encodeBase64URL(v.clientDataJSON),attestationObject:encodeBase64URL(v.attestationObject),transports:v.getTransports?.()||[]};}
 else{const v=response as AuthenticatorAssertionResponse;result.response={clientDataJSON:encodeBase64URL(v.clientDataJSON),authenticatorData:encodeBase64URL(v.authenticatorData),signature:encodeBase64URL(v.signature),userHandle:v.userHandle?encodeBase64URL(v.userHandle):null};}
 return result;
}
