// +build !ignore_autogenerated

/*
Copyright © 2021 BoxBoat

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
// Code generated by main. DO NOT EDIT.

package v1alpha1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AwsSecretsManager) DeepCopyInto(out *AwsSecretsManager) {
	*out = *in
	if in.AccessKeyId != nil {
		in, out := &in.AccessKeyId, &out.AccessKeyId
		*out = new(string)
		**out = **in
	}
	if in.SecretAccessKeyRef != nil {
		in, out := &in.SecretAccessKeyRef, &out.SecretAccessKeyRef
		*out = new(string)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AwsSecretsManager.
func (in *AwsSecretsManager) DeepCopy() *AwsSecretsManager {
	if in == nil {
		return nil
	}
	out := new(AwsSecretsManager)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AzureKeyVault) DeepCopyInto(out *AzureKeyVault) {
	*out = *in
	if in.ClientId != nil {
		in, out := &in.ClientId, &out.ClientId
		*out = new(string)
		**out = **in
	}
	if in.ClientSecretRef != nil {
		in, out := &in.ClientSecretRef, &out.ClientSecretRef
		*out = new(string)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AzureKeyVault.
func (in *AzureKeyVault) DeepCopy() *AzureKeyVault {
	if in == nil {
		return nil
	}
	out := new(AzureKeyVault)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DockhandProfile) DeepCopyInto(out *DockhandProfile) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.AwsSecretsManager != nil {
		in, out := &in.AwsSecretsManager, &out.AwsSecretsManager
		*out = new(AwsSecretsManager)
		(*in).DeepCopyInto(*out)
	}
	if in.AzureKeyVault != nil {
		in, out := &in.AzureKeyVault, &out.AzureKeyVault
		*out = new(AzureKeyVault)
		(*in).DeepCopyInto(*out)
	}
	if in.GcpSecretsManager != nil {
		in, out := &in.GcpSecretsManager, &out.GcpSecretsManager
		*out = new(GcpSecretsManager)
		(*in).DeepCopyInto(*out)
	}
	if in.Vault != nil {
		in, out := &in.Vault, &out.Vault
		*out = new(Vault)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DockhandProfile.
func (in *DockhandProfile) DeepCopy() *DockhandProfile {
	if in == nil {
		return nil
	}
	out := new(DockhandProfile)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DockhandProfile) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DockhandProfileList) DeepCopyInto(out *DockhandProfileList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]DockhandProfile, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DockhandProfileList.
func (in *DockhandProfileList) DeepCopy() *DockhandProfileList {
	if in == nil {
		return nil
	}
	out := new(DockhandProfileList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DockhandProfileList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DockhandSecret) DeepCopyInto(out *DockhandSecret) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.Data != nil {
		in, out := &in.Data, &out.Data
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	out.Status = in.Status
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DockhandSecret.
func (in *DockhandSecret) DeepCopy() *DockhandSecret {
	if in == nil {
		return nil
	}
	out := new(DockhandSecret)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DockhandSecret) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DockhandSecretList) DeepCopyInto(out *DockhandSecretList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]DockhandSecret, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DockhandSecretList.
func (in *DockhandSecretList) DeepCopy() *DockhandSecretList {
	if in == nil {
		return nil
	}
	out := new(DockhandSecretList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DockhandSecretList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DockhandSecretStatus) DeepCopyInto(out *DockhandSecretStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DockhandSecretStatus.
func (in *DockhandSecretStatus) DeepCopy() *DockhandSecretStatus {
	if in == nil {
		return nil
	}
	out := new(DockhandSecretStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GcpSecretsManager) DeepCopyInto(out *GcpSecretsManager) {
	*out = *in
	if in.CredentialsFileSecretRef != nil {
		in, out := &in.CredentialsFileSecretRef, &out.CredentialsFileSecretRef
		*out = new(string)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GcpSecretsManager.
func (in *GcpSecretsManager) DeepCopy() *GcpSecretsManager {
	if in == nil {
		return nil
	}
	out := new(GcpSecretsManager)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Vault) DeepCopyInto(out *Vault) {
	*out = *in
	if in.RoleId != nil {
		in, out := &in.RoleId, &out.RoleId
		*out = new(string)
		**out = **in
	}
	if in.SecretIdRef != nil {
		in, out := &in.SecretIdRef, &out.SecretIdRef
		*out = new(string)
		**out = **in
	}
	if in.TokenRef != nil {
		in, out := &in.TokenRef, &out.TokenRef
		*out = new(string)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Vault.
func (in *Vault) DeepCopy() *Vault {
	if in == nil {
		return nil
	}
	out := new(Vault)
	in.DeepCopyInto(out)
	return out
}
