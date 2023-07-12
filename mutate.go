package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/api/admission/v1"
	networking_v1 "k8s.io/api/networking/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// the struct the patch needs for each image
type patchOperation struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value"`
}

var coreClient corev1.CoreV1Interface

// Create mutate function which can be called from multiplexer in main.go
func (srvstrc *myServerHandler) mutserve(w http.ResponseWriter, r *http.Request) {

	timestampLog := Log()

	var patchData []patchOperation
	// Create structure for capturing and processing body captured from webhooks
	var Body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			Body = data
		}
	}
	// Error handling for not capturing data
	if len(Body) == 0 {
		timestampLog.Errorf("Unable to retrieve body from API request")
		http.Error(w, "Empty Body", http.StatusBadRequest)
	}

	// Read the Response from the Kubernetes API and place it in the Request
	arRequest := &v1.AdmissionReview{}
	err = json.Unmarshal(Body, arRequest)
	if err != nil {
		timestampLog.Errorf("Error unmarshelling the body request")
		http.Error(w, "Error Unmarsheling the Body request", http.StatusBadRequest)
		return
	}
	// Specifying Ingress object from data body
	raw := arRequest.Request.Object.Raw
	obj := networking_v1.Ingress{}

	err = json.Unmarshal(raw, &obj)
	if err != nil {
		timestampLog.Errorf("Error unmarshelling the body request")
		http.Error(w, "Error Unmarsheling the Body request", http.StatusBadRequest)
		return
	}

	arResponse := v1.AdmissionReview{
		Response: &v1.AdmissionResponse{
			UID: arRequest.Request.UID,
		},
	}
// Conditional checks to ensure fields exist in Ingress object
	if obj.Spec.Rules != nil {

		if obj.Spec.Rules[0].HTTP != nil {
// Create full host name variable to be used for patching/mutating
			fullHostName := obj.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name + "-" + obj.Namespace + ".apps." + UrlSuffix.Spec.BaseDomain
// JSON patching to execute if conditions aren't met
			if obj.Spec.Rules[0].Host != fullHostName {
				patchData = append(patchData, patchOperation{
					Op:    "replace",
					Path:  "/spec/rules/0/host",
					Value: fullHostName})
			}
			if obj.Spec.TLS != nil {

				if obj.Spec.TLS[0].Hosts != nil {

					if obj.Spec.TLS[0].Hosts[0] != fullHostName {
						patchData = append(patchData, patchOperation{
							Op:    "replace",
							Path:  "/spec/tls/0/hosts/0",
							Value: fullHostName})
					}
				}
			}
// More JSON patching for the necessary annotations to be mutated for cert-manager to pick up the Ingress object
			if obj.ObjectMeta.Annotations != nil {
				patchData = append(patchData, patchOperation{
					Op:    "replace",
					Path:  "/metadata/annotations/" + "cert-manager.io~1issuer",
					Value: issuer})

				patchData = append(patchData, patchOperation{
					Op:    "replace",
					Path:  "/metadata/annotations/" + "cert-manager.io~1issuer-kind",
					Value: issuerKind})

				patchData = append(patchData, patchOperation{
					Op:    "replace",
					Path:  "/metadata/annotations/" + "cert-manager.io~1issuer-group",
					Value: issuerGroup})

				patchData = append(patchData, patchOperation{
					Op:    "replace",
					Path:  "/metadata/annotations/" + "cert-manager.io~1tls-acme",
					Value: "false"})

				patchData = append(patchData, patchOperation{
					Op:    "replace",
					Path:  "/metadata/annotations/" + "cert-manager.io~1common-name",
					Value: fullHostName})
			}
		}
	}  
// Encoding the built up the mutated response so it can be sent back to API server
	patchBytes, err := json.Marshal(patchData)
	if err != nil {
		timestampLog.Errorf("Can't encode response %v", err)
		http.Error(w, fmt.Sprintf("couldn't encode Patches: %v", err), http.StatusInternalServerError)
		return
	}
	v1JSONPatch := admissionv1.PatchTypeJSONPatch
	arResponse.APIVersion = "admission.k8s.io/v1"
	arResponse.Kind = arRequest.Kind
	arResponse.Response.Allowed = true
	arResponse.Response.Patch = patchBytes
	arResponse.Response.PatchType = &v1JSONPatch
// More error handling
	resp, err := json.Marshal(arResponse)
	if err != nil {
		timestampLog.Errorf("Can't encode response %v", err)
		http.Error(w, fmt.Sprintf("couldn't encode response: %v", err), http.StatusInternalServerError)
	}

	_, err = w.Write(resp)
	if err != nil {
		timestampLog.Errorf("Can't write response %v", err)
		http.Error(w, fmt.Sprintf("cloud not write response: %v", err), http.StatusInternalServerError)
	}
}
