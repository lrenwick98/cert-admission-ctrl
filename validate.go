package main

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	networking_v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Create validate function to be called from main.go
func (gs *myServerHandler) valserve(w http.ResponseWriter, r *http.Request) {

	timestampLog := Log()

	var Body []byte
	// Retrieving auth from main so logic can be executed within OCP
	config, err := GetConfig()
	if err != nil {
		log.Fatal(err)
	}

	coreClientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
	}

	coreClient := coreClientSet.CoreV1()

	ctx := context.TODO()

	var errorMessage string
	// Declaring variables so validation can be checked against
	serviceCorrect := false
	hostCorrect := false
	tlsCorrect := false
	annotationCorrect := false
	secretExist := false
	// Error handling for capturing body of request
	if r.Body != nil {
		data, err := ioutil.ReadAll(r.Body)
		if err == nil {
			Body = data
		}
	}
	// Error handling
	if len(Body) == 0 {
		timestampLog.Errorf("Unable to retrieve Body from webhook: %v", http.StatusBadRequest)
		http.Error(w, "Unable to retrieve Body from the API", http.StatusBadRequest)
		return
	}

	arRequest := &admissionv1.AdmissionReview{}
	//  Unmarshalling of request
	err = json.Unmarshal(Body, arRequest)
	if err != nil {
		timestampLog.Errorf("Unable to unmarshal the request: %v", http.StatusBadRequest)
		http.Error(w, "unable to Unmarshal the Body Request", http.StatusBadRequest)
		return
	}

	// initial the POD values from the request
	raw := arRequest.Request.Object.Raw
	obj := networking_v1.Ingress{}

	if err := json.Unmarshal(raw, &obj); err != nil {
		timestampLog.Errorf("Unable to unmarshal the request: %v", http.StatusBadRequest)
		http.Error(w, "Unable to Unmarshal the Pod Information", http.StatusBadRequest)
	}
	// Creating variable of retrieving list of existing services for given namespace
	svcList, err := coreClient.Services(obj.Namespace).List(ctx, metav1.ListOptions{})

	svcNameIngress := obj.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name
	// Loop to check service name in Ingress matches up to one of those found in given namespace
	for _, services := range svcList.Items {
		if strings.Contains(svcNameIngress, services.ObjectMeta.Name) {
			serviceCorrect = true
		} else {
			errorMessage = "Make sure service name matches up to an existing service in the namespace"
		}
	}
	// Validation checks for correct fields and values to exists within Ingress object
	if obj.Spec.Rules != nil {

		if obj.Spec.Rules[0].HTTP != nil {

			fullHostName := obj.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name + "-" + obj.Namespace + ".apps." + UrlSuffix.Spec.BaseDomain

			if obj.Spec.Rules[0].Host == fullHostName {
				hostCorrect = true
			} else {
				errorMessage = "Hostname isn't the same"
			}
			// More validation checks and creation of error messages to display to user if condition is not met
			if obj.Spec.TLS != nil {

				if obj.Spec.TLS[0].Hosts != nil {
					if obj.Spec.TLS[0].SecretName != "" {
						secretExist = true
					} else {
						errorMessage = "Make sure you specify secretName under tls then hosts"
					}
					if obj.Spec.TLS[0].Hosts[0] == fullHostName {
						tlsCorrect = true
					} else {
						errorMessage = "TLS hostname isn't the same"
					}
				} else {
					errorMessage = "TLS Hosts is missing"
				}
			} else {
				errorMessage = "TLS is not specified"
			}
		} else {
			errorMessage = "Incorrect ingress spec rules"
		}
	} else {
		errorMessage = "Ingress spec is incorrect"
	}

	if obj.ObjectMeta.Annotations != nil {
		annotationCorrect = true
	} else {
		errorMessage = "Annotations don't exist. Add annotation 'route.openshift.io/termination: <edge,passthrough>'"
	}
	// Building of admission review response
	arResponse := admissionv1.AdmissionReview{
		Response: &admissionv1.AdmissionResponse{
			Result:  &metav1.Status{Status: "Failure", Message: errorMessage, Code: 406},
			UID:     arRequest.Request.UID,
			Allowed: false,
		},
	}
	// Logic to not send back to server with given error message if all conditional variables are not satisfied
	arResponse.APIVersion = "admission.k8s.io/v1"
	arResponse.Kind = arRequest.Kind
	if tlsCorrect && hostCorrect && annotationCorrect && secretExist && serviceCorrect {
		arResponse.Response.Allowed = true
		arResponse.Response.Result = &metav1.Status{Status: "Success",
			Message: "All conditions have been met and are validated",
			Code:    201}
	}
	// Error handling for marshalling response and sending back to server
	resp, err := json.Marshal(arResponse)
	if err != nil {
		timestampLog.Errorf("Unable to marshal the request: %v", http.StatusBadRequest)
		http.Error(w, "Unable to Marshal the Request", http.StatusBadRequest)
	}

	_, err = w.Write(resp)
	if err != nil {
		timestampLog.Errorf("Unable to write the response, HTTP error: %v", http.StatusBadRequest)
		http.Error(w, "Unable to Write Response", http.StatusBadRequest)
	} else {
		timestampLog.Infof("Ingress was created/changed in namespace: %s, By user: %s, User is in groups: %s, Ingress hostname is: %s", arRequest.Request.Namespace, arRequest.Request.UserInfo.Username, arRequest.Request.UserInfo.Groups, obj.Spec.Rules[0].Host)

	}
}
